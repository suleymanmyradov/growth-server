package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"google.golang.org/grpc/codes"
)

// ============================================
// Test helpers for Redis-backed tests
// ============================================

// setupMiniredis starts an in-memory Redis server and returns a client + cleanup.
func setupMiniredis(t *testing.T) (*redis.Client, func()) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	cleanup := func() {
		_ = client.Close()
		mr.Close()
	}
	return client, cleanup
}

// ============================================
// RegisterLogic full unit tests
// ============================================

func TestRegisterLogic_Success(t *testing.T) {
	ctx := context.Background()
	redisClient, cleanup := setupMiniredis(t)
	defer cleanup()

	mockUsers := &MockUsersRepo{}
	mockTxRunner := &MockTxRunner{}
	mockEmailSender := &MockEmailSender{}

	mockTxRunner.On("Run", mock.Anything, "", mock.AnythingOfType("func(pgx.Tx) error")).Return(nil)
	mockEmailSender.On("Send", mock.Anything, mock.Anything).Return(nil)

	svcCtx := &svc.ServiceContext{
		Repo:        &repository.Repository{Users: mockUsers},
		TxRunner:    mockTxRunner,
		RedisClient: redisClient,
		EmailSender: mockEmailSender,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewRegisterLogic(ctx, svcCtx)
	resp, err := l.Register(&auth.RegisterRequest{
		Username: "janedoe",
		Email:    "jane@example.com",
		Password: "Abcdef1!",
		FullName: "Jane Doe",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.True(t, resp.RequiresVerification)
	assert.Contains(t, resp.Message, "Check your email")
}

func TestRegisterLogic_DuplicateUser(t *testing.T) {
	ctx := context.Background()
	redisClient, cleanup := setupMiniredis(t)
	defer cleanup()

	mockTxRunner := &MockTxRunner{}

	// The register logic wraps the pgErr inside the transaction callback as
	// ErrUserAlreadyExists. Since the mock TxRunner doesn't call the callback,
	// we return the wrapped error directly.
	mockTxRunner.On("Run", mock.Anything, "", mock.AnythingOfType("func(pgx.Tx) error")).
		Return(ErrUserAlreadyExists)

	svcCtx := &svc.ServiceContext{
		TxRunner:    mockTxRunner,
		RedisClient: redisClient,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewRegisterLogic(ctx, svcCtx)
	_, err := l.Register(&auth.RegisterRequest{
		Username: "janedoe",
		Email:    "jane@example.com",
		Password: "Abcdef1!",
		FullName: "Jane Doe",
	})

	assertGrpcError(t, err, codes.AlreadyExists, MsgUserAlreadyExists)
}

// ============================================
// VerifyEmailLogic full unit tests
// ============================================

func TestVerifyEmailLogic_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	redisClient, cleanup := setupMiniredis(t)
	defer cleanup()

	// Store a verification token in miniredis
	verificationRepo := repository.NewVerificationRepo(redisClient)
	user := makeTestUser(func(u *db.User) { u.ID = userID; u.EmailVerified = false })
	require.NoError(t, verificationRepo.Store(ctx, "valid-token", userID.String(), user.Email, time.Hour))

	mockUsers := &MockUsersRepo{}
	mockTokenMaker := &MockTokenMaker{}

	verifiedUser := makeTestUser(func(u *db.User) { u.ID = userID; u.EmailVerified = true })
	mockUsers.On("SetEmailVerified", mock.Anything, userID).Return(verifiedUser, nil)
	mockTokenMaker.On("CreateAccessToken", mock.Anything, userID, "janedoe", []string{"user"}, mock.AnythingOfType("uuid.UUID")).
		Return(&jwt.TokenResponse{Token: "access-token"}, nil)
	mockTokenMaker.On("CreateRefreshToken", mock.Anything, userID, "janedoe", []string{"user"}, mock.AnythingOfType("uuid.UUID")).
		Return(&jwt.TokenResponse{Token: "refresh-token"}, nil)

	svcCtx := &svc.ServiceContext{
		Repo:        &repository.Repository{Users: mockUsers},
		TokenMaker:  mockTokenMaker,
		RedisClient: redisClient,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewVerifyEmailLogic(ctx, svcCtx)
	resp, err := l.VerifyEmail(&auth.VerifyEmailRequest{Token: "valid-token"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "access-token", resp.AccessToken)
	assert.Equal(t, "refresh-token", resp.RefreshToken)
	assert.True(t, resp.User.EmailVerified)
}

func TestVerifyEmailLogic_InvalidToken(t *testing.T) {
	ctx := context.Background()
	redisClient, cleanup := setupMiniredis(t)
	defer cleanup()

	svcCtx := &svc.ServiceContext{
		RedisClient: redisClient,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewVerifyEmailLogic(ctx, svcCtx)
	_, err := l.VerifyEmail(&auth.VerifyEmailRequest{Token: "nonexistent-token"})

	assertGrpcError(t, err, codes.Unauthenticated, MsgInvalidOrExpiredVerifToken)
}

// ============================================
// ResetPasswordLogic full unit tests
// ============================================

func TestResetPasswordLogic_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	redisClient, cleanup := setupMiniredis(t)
	defer cleanup()

	// Store a reset token in miniredis
	resetRepo := repository.NewPasswordResetRepo(redisClient)
	require.NoError(t, resetRepo.Store(ctx, "valid-reset-token", "jane@example.com", time.Hour))

	user := makeTestUser(func(u *db.User) { u.ID = userID })
	mockUsers := &MockUsersRepo{}
	mockUsers.On("GetUserByEmail", mock.Anything, "jane@example.com").Return(user, nil)
	mockUsers.On("UpdateUserPassword", mock.Anything, userID, mock.Anything).Return(user, nil)

	svcCtx := &svc.ServiceContext{
		Repo:        &repository.Repository{Users: mockUsers},
		RedisClient: redisClient,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewResetPasswordLogic(ctx, svcCtx)
	_, err := l.ResetPassword(&auth.ResetPasswordRequest{
		Token:       "valid-reset-token",
		NewPassword: "NewPass1!",
	})

	require.NoError(t, err)
}

func TestResetPasswordLogic_InvalidToken(t *testing.T) {
	ctx := context.Background()
	redisClient, cleanup := setupMiniredis(t)
	defer cleanup()

	svcCtx := &svc.ServiceContext{
		RedisClient: redisClient,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewResetPasswordLogic(ctx, svcCtx)
	_, err := l.ResetPassword(&auth.ResetPasswordRequest{
		Token:       "nonexistent-token",
		NewPassword: "NewPass1!",
	})

	assertGrpcError(t, err, codes.Unauthenticated, MsgInvalidOrExpiredResetToken)
}

func TestResetPasswordLogic_UserNotFound(t *testing.T) {
	ctx := context.Background()
	redisClient, cleanup := setupMiniredis(t)
	defer cleanup()

	// Store a reset token for an email that doesn't exist in the DB
	resetRepo := repository.NewPasswordResetRepo(redisClient)
	require.NoError(t, resetRepo.Store(ctx, "valid-reset-token", "ghost@example.com", time.Hour))

	mockUsers := &MockUsersRepo{}
	mockUsers.On("GetUserByEmail", mock.Anything, "ghost@example.com").Return(db.User{}, errors.New("no rows"))

	svcCtx := &svc.ServiceContext{
		Repo:        &repository.Repository{Users: mockUsers},
		RedisClient: redisClient,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewResetPasswordLogic(ctx, svcCtx)
	_, err := l.ResetPassword(&auth.ResetPasswordRequest{
		Token:       "valid-reset-token",
		NewPassword: "NewPass1!",
	})

	assertGrpcError(t, err, codes.NotFound, MsgUserNotFound)
}

// ============================================
// ForgotPasswordLogic full unit tests
// ============================================

func TestForgotPasswordLogic_Success(t *testing.T) {
	ctx := context.Background()
	redisClient, cleanup := setupMiniredis(t)
	defer cleanup()

	user := makeTestUser()
	mockUsers := &MockUsersRepo{}
	mockEmailSender := &MockEmailSender{}
	mockUsers.On("GetUserByEmail", mock.Anything, "jane@example.com").Return(user, nil)
	mockEmailSender.On("Send", mock.Anything, mock.Anything).Return(nil)

	svcCtx := &svc.ServiceContext{
		Repo:        &repository.Repository{Users: mockUsers},
		RedisClient: redisClient,
		EmailSender: mockEmailSender,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewForgotPasswordLogic(ctx, svcCtx)
	resp, err := l.ForgotPassword(&auth.ForgotPasswordRequest{Email: "jane@example.com"})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	// Email should have been sent
	assert.Len(t, mockEmailSender.Sent, 1)
	assert.Contains(t, mockEmailSender.Sent[0].Subject, "Reset your password")
}

func TestForgotPasswordLogic_UserNotFound_ReturnsSuccess(t *testing.T) {
	ctx := context.Background()
	redisClient, cleanup := setupMiniredis(t)
	defer cleanup()

	mockUsers := &MockUsersRepo{}
	mockUsers.On("GetUserByEmail", mock.Anything, "unknown@example.com").Return(db.User{}, errors.New("no rows"))

	svcCtx := &svc.ServiceContext{
		Repo:        &repository.Repository{Users: mockUsers},
		RedisClient: redisClient,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewForgotPasswordLogic(ctx, svcCtx)
	resp, err := l.ForgotPassword(&auth.ForgotPasswordRequest{Email: "unknown@example.com"})

	// Should return success (no leak) even if user doesn't exist
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

// ============================================
// ResendVerificationLogic full unit tests
// ============================================

func TestResendVerificationLogic_Success(t *testing.T) {
	ctx := context.Background()
	redisClient, cleanup := setupMiniredis(t)
	defer cleanup()

	user := makeTestUser(withEmailVerified(false))
	mockUsers := &MockUsersRepo{}
	mockEmailSender := &MockEmailSender{}
	mockUsers.On("GetUserByEmail", mock.Anything, "jane@example.com").Return(user, nil)
	mockEmailSender.On("Send", mock.Anything, mock.Anything).Return(nil)

	svcCtx := &svc.ServiceContext{
		Repo:        &repository.Repository{Users: mockUsers},
		RedisClient: redisClient,
		EmailSender: mockEmailSender,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewResendVerificationLogic(ctx, svcCtx)
	resp, err := l.ResendVerification(&auth.ResendVerificationRequest{Email: "jane@example.com"})

	require.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Len(t, mockEmailSender.Sent, 1)
}

func TestResendVerificationLogic_AlreadyVerified(t *testing.T) {
	ctx := context.Background()
	redisClient, cleanup := setupMiniredis(t)
	defer cleanup()

	user := makeTestUser(withEmailVerified(true))
	mockUsers := &MockUsersRepo{}
	mockUsers.On("GetUserByEmail", mock.Anything, "jane@example.com").Return(user, nil)

	svcCtx := &svc.ServiceContext{
		Repo:        &repository.Repository{Users: mockUsers},
		RedisClient: redisClient,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewResendVerificationLogic(ctx, svcCtx)
	resp, err := l.ResendVerification(&auth.ResendVerificationRequest{Email: "jane@example.com"})

	// Should return success (no-op) for already-verified emails
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestResendVerificationLogic_UserNotFound(t *testing.T) {
	ctx := context.Background()
	redisClient, cleanup := setupMiniredis(t)
	defer cleanup()

	mockUsers := &MockUsersRepo{}
	mockUsers.On("GetUserByEmail", mock.Anything, "unknown@example.com").Return(db.User{}, errors.New("no rows"))

	svcCtx := &svc.ServiceContext{
		Repo:        &repository.Repository{Users: mockUsers},
		RedisClient: redisClient,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewResendVerificationLogic(ctx, svcCtx)
	resp, err := l.ResendVerification(&auth.ResendVerificationRequest{Email: "unknown@example.com"})

	// Should return success (no leak) even if user doesn't exist
	require.NoError(t, err)
	assert.NotNil(t, resp)
}

func TestResendVerificationLogic_Throttled(t *testing.T) {
	ctx := context.Background()
	redisClient, cleanup := setupMiniredis(t)
	defer cleanup()

	user := makeTestUser(withEmailVerified(false))

	// Set a pending throttle so the resend is blocked
	verificationRepo := repository.NewVerificationRepo(redisClient)
	require.NoError(t, verificationRepo.SetThrottle(ctx, user.Email, 60*time.Second))

	mockUsers := &MockUsersRepo{}
	mockUsers.On("GetUserByEmail", mock.Anything, "jane@example.com").Return(user, nil)

	svcCtx := &svc.ServiceContext{
		Repo:        &repository.Repository{Users: mockUsers},
		RedisClient: redisClient,
		Config:      testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewResendVerificationLogic(ctx, svcCtx)
	_, err := l.ResendVerification(&auth.ResendVerificationRequest{Email: "jane@example.com"})

	assertGrpcError(t, err, codes.ResourceExhausted, MsgPleaseWaitBeforeResend)
}

// ============================================
// LogoutLogic full unit tests
// ============================================

func TestLogoutLogic_Success(t *testing.T) {
	ctx := context.Background()
	sessionID := uuid.New()
	userID := uuid.New()

	mockTokenMaker := &MockTokenMaker{}
	mockTokenMaker.On("VerifyAccessToken", mock.Anything, "valid-access-token").
		Return(&jwt.TokenClaims{Subject: userID, SessionID: sessionID}, nil)
	mockTokenMaker.On("RevokeAccessToken", mock.Anything, "valid-access-token").Return(nil)
	mockTokenMaker.On("RevokeSession", mock.Anything, sessionID, mock.AnythingOfType("time.Duration")).Return(nil)
	mockTokenMaker.On("RevokeRefreshToken", mock.Anything, "valid-refresh-token").Return(nil)

	svcCtx := &svc.ServiceContext{
		TokenMaker: mockTokenMaker,
		Config:     testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewLogoutLogic(ctx, svcCtx)
	_, err := l.Logout(&auth.LogoutRequest{
		AccessToken:  "valid-access-token",
		RefreshToken: "valid-refresh-token",
	})

	require.NoError(t, err)
}

func TestLogoutLogic_InvalidToken(t *testing.T) {
	ctx := context.Background()
	mockTokenMaker := &MockTokenMaker{}
	mockTokenMaker.On("VerifyAccessToken", mock.Anything, "bad-token").
		Return(nil, errors.New("invalid token"))

	svcCtx := &svc.ServiceContext{
		TokenMaker: mockTokenMaker,
		Config:     testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewLogoutLogic(ctx, svcCtx)
	_, err := l.Logout(&auth.LogoutRequest{AccessToken: "bad-token"})

	assertGrpcError(t, err, codes.Unauthenticated, MsgInvalidToken)
}

func TestLogoutLogic_SessionRevocationFails(t *testing.T) {
	ctx := context.Background()
	sessionID := uuid.New()
	userID := uuid.New()

	mockTokenMaker := &MockTokenMaker{}
	mockTokenMaker.On("VerifyAccessToken", mock.Anything, "valid-access-token").
		Return(&jwt.TokenClaims{Subject: userID, SessionID: sessionID}, nil)
	mockTokenMaker.On("RevokeAccessToken", mock.Anything, "valid-access-token").Return(nil)
	mockTokenMaker.On("RevokeSession", mock.Anything, sessionID, mock.AnythingOfType("time.Duration")).
		Return(errors.New("redis down"))

	svcCtx := &svc.ServiceContext{
		TokenMaker: mockTokenMaker,
		Config:     testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewLogoutLogic(ctx, svcCtx)
	_, err := l.Logout(&auth.LogoutRequest{AccessToken: "valid-access-token"})

	// Session revocation failure is a critical security error — must return error
	assertGrpcError(t, err, codes.Internal, MsgFailedRevokeSession)
}

func TestLogoutLogic_AccessTokenRevocationFails_StillSucceeds(t *testing.T) {
	ctx := context.Background()
	sessionID := uuid.New()
	userID := uuid.New()

	mockTokenMaker := &MockTokenMaker{}
	mockTokenMaker.On("VerifyAccessToken", mock.Anything, "valid-access-token").
		Return(&jwt.TokenClaims{Subject: userID, SessionID: sessionID}, nil)
	mockTokenMaker.On("RevokeAccessToken", mock.Anything, "valid-access-token").
		Return(errors.New("best-effort failure"))
	mockTokenMaker.On("RevokeSession", mock.Anything, sessionID, mock.AnythingOfType("time.Duration")).Return(nil)

	svcCtx := &svc.ServiceContext{
		TokenMaker: mockTokenMaker,
		Config:     testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewLogoutLogic(ctx, svcCtx)
	_, err := l.Logout(&auth.LogoutRequest{AccessToken: "valid-access-token"})

	// Access token revocation is best-effort — logout should still succeed
	require.NoError(t, err)
}

// ============================================
// RefreshTokenLogic full unit tests
// ============================================

func TestRefreshTokenLogic_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()
	user := makeTestUser(func(u *db.User) { u.ID = userID })

	mockUsers := &MockUsersRepo{}
	mockTokenMaker := &MockTokenMaker{}

	mockTokenMaker.On("VerifyRefreshToken", mock.Anything, "valid-refresh-token").
		Return(&jwt.TokenClaims{Subject: userID, SessionID: sessionID}, nil)
	mockTokenMaker.On("IsSessionRevoked", mock.Anything, sessionID).Return(false, nil)
	mockUsers.On("GetUserByID", mock.Anything, userID).Return(user, nil)
	mockTokenMaker.On("CreateAccessToken", mock.Anything, userID, "janedoe", []string{"user"}, sessionID).
		Return(&jwt.TokenResponse{Token: "new-access-token"}, nil)
	mockTokenMaker.On("RotateRefreshToken", mock.Anything, "valid-refresh-token").
		Return(&jwt.TokenResponse{Token: "new-refresh-token"}, nil)

	svcCtx := &svc.ServiceContext{
		Repo:       &repository.Repository{Users: mockUsers},
		TokenMaker: mockTokenMaker,
		Config:     testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewRefreshTokenLogic(ctx, svcCtx)
	resp, err := l.RefreshToken(&auth.RefreshRequest{RefreshToken: "valid-refresh-token"})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "new-access-token", resp.AccessToken)
	assert.Equal(t, "new-refresh-token", resp.RefreshToken)
}

func TestRefreshTokenLogic_InvalidRefreshToken(t *testing.T) {
	ctx := context.Background()
	mockTokenMaker := &MockTokenMaker{}
	mockTokenMaker.On("VerifyRefreshToken", mock.Anything, "bad-token").
		Return(nil, errors.New("invalid"))

	svcCtx := &svc.ServiceContext{
		TokenMaker: mockTokenMaker,
		Config:     testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewRefreshTokenLogic(ctx, svcCtx)
	_, err := l.RefreshToken(&auth.RefreshRequest{RefreshToken: "bad-token"})

	assertGrpcError(t, err, codes.Unauthenticated, MsgInvalidOrExpiredRefreshToken)
}

func TestRefreshTokenLogic_SessionRevoked(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()

	mockTokenMaker := &MockTokenMaker{}
	mockTokenMaker.On("VerifyRefreshToken", mock.Anything, "valid-refresh-token").
		Return(&jwt.TokenClaims{Subject: userID, SessionID: sessionID}, nil)
	mockTokenMaker.On("IsSessionRevoked", mock.Anything, sessionID).Return(true, nil)

	svcCtx := &svc.ServiceContext{
		TokenMaker: mockTokenMaker,
		Config:     testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewRefreshTokenLogic(ctx, svcCtx)
	_, err := l.RefreshToken(&auth.RefreshRequest{RefreshToken: "valid-refresh-token"})

	assertGrpcError(t, err, codes.Unauthenticated, MsgSessionRevoked)
}

func TestRefreshTokenLogic_UserNotFound(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	sessionID := uuid.New()

	mockTokenMaker := &MockTokenMaker{}
	mockUsers := &MockUsersRepo{}

	mockTokenMaker.On("VerifyRefreshToken", mock.Anything, "valid-refresh-token").
		Return(&jwt.TokenClaims{Subject: userID, SessionID: sessionID}, nil)
	mockTokenMaker.On("IsSessionRevoked", mock.Anything, sessionID).Return(false, nil)
	mockUsers.On("GetUserByID", mock.Anything, userID).Return(db.User{}, errors.New("no rows"))

	svcCtx := &svc.ServiceContext{
		Repo:       &repository.Repository{Users: mockUsers},
		TokenMaker: mockTokenMaker,
		Config:     testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewRefreshTokenLogic(ctx, svcCtx)
	_, err := l.RefreshToken(&auth.RefreshRequest{RefreshToken: "valid-refresh-token"})

	assertGrpcError(t, err, codes.NotFound, MsgUserNotFound)
}
