package logic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
)

// ============================================
// LoginLogic full unit tests
// ============================================

func TestLoginLogic_Success(t *testing.T) {
	ctx := context.Background()
	userID := uuid.New()
	hash, err := bcrypt.GenerateFromPassword([]byte("Abcdef1!"), bcrypt.MinCost)
	require.NoError(t, err)
	user := makeTestUser(func(u *db.User) {
		u.ID = userID
		u.PasswordHash = strPtr(string(hash))
		u.EmailVerified = true
	})

	mockUsers := &MockUsersRepo{}
	mockTokenMaker := &MockTokenMaker{}

	mockUsers.On("GetUserByEmail", mock.Anything, "jane@example.com").Return(user, nil)
	mockTokenMaker.On("CreateAccessToken", mock.Anything, userID, "janedoe", []string{"user"}, mock.AnythingOfType("uuid.UUID")).
		Return(&jwt.TokenResponse{Token: "access-token", ExpiresAt: time.Now().Add(15 * time.Minute)}, nil)
	mockTokenMaker.On("CreateRefreshToken", mock.Anything, userID, "janedoe", []string{"user"}, mock.AnythingOfType("uuid.UUID")).
		Return(&jwt.TokenResponse{Token: "refresh-token", ExpiresAt: time.Now().Add(7 * 24 * time.Hour)}, nil)

	svcCtx := &svc.ServiceContext{
		Repo:       &repository.Repository{Users: mockUsers},
		TokenMaker: mockTokenMaker,
		Config:     testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewLoginLogic(ctx, svcCtx)
	resp, err := l.Login(&auth.LoginRequest{
		Email:    "jane@example.com",
		Password: "Abcdef1!",
	})

	require.NoError(t, err)
	require.NotNil(t, resp)
	assert.Equal(t, "access-token", resp.AccessToken)
	assert.Equal(t, "refresh-token", resp.RefreshToken)
	assert.Equal(t, int64(900), resp.ExpiresIn) // 15 min = 900s
	assert.Equal(t, "jane@example.com", resp.User.Email)
}

func TestLoginLogic_UserNotFound(t *testing.T) {
	ctx := context.Background()
	mockUsers := &MockUsersRepo{}
	mockUsers.On("GetUserByEmail", mock.Anything, "unknown@example.com").Return(db.User{}, errors.New("no rows"))

	svcCtx := &svc.ServiceContext{
		Repo: &repository.Repository{Users: mockUsers},
	}

	l := NewLoginLogic(ctx, svcCtx)
	_, err := l.Login(&auth.LoginRequest{
		Email:    "unknown@example.com",
		Password: "Abcdef1!",
	})

	assertGrpcError(t, err, codes.Unauthenticated, MsgInvalidCredentials)
}

func TestLoginLogic_OAuthOnlyAccount(t *testing.T) {
	ctx := context.Background()
	user := makeTestUser(withPasswordHash(nil)) // no password hash = OAuth-only

	mockUsers := &MockUsersRepo{}
	mockUsers.On("GetUserByEmail", mock.Anything, "jane@example.com").Return(user, nil)

	svcCtx := &svc.ServiceContext{
		Repo: &repository.Repository{Users: mockUsers},
	}

	l := NewLoginLogic(ctx, svcCtx)
	_, err := l.Login(&auth.LoginRequest{
		Email:    "jane@example.com",
		Password: "Abcdef1!",
	})

	assertGrpcError(t, err, codes.Unauthenticated, MsgInvalidCredentials)
}

func TestLoginLogic_WrongPassword(t *testing.T) {
	ctx := context.Background()
	hash, err := bcrypt.GenerateFromPassword([]byte("CorrectPass1!"), bcrypt.MinCost)
	require.NoError(t, err)
	user := makeTestUser(withPasswordHash(strPtr(string(hash))))

	mockUsers := &MockUsersRepo{}
	mockUsers.On("GetUserByEmail", mock.Anything, "jane@example.com").Return(user, nil)

	svcCtx := &svc.ServiceContext{
		Repo: &repository.Repository{Users: mockUsers},
	}

	l := NewLoginLogic(ctx, svcCtx)
	_, err = l.Login(&auth.LoginRequest{
		Email:    "jane@example.com",
		Password: "WrongPassword1!",
	})

	assertGrpcError(t, err, codes.Unauthenticated, MsgInvalidCredentials)
}

func TestLoginLogic_EmailNotVerified(t *testing.T) {
	ctx := context.Background()
	hash, err := bcrypt.GenerateFromPassword([]byte("Abcdef1!"), bcrypt.MinCost)
	require.NoError(t, err)
	user := makeTestUser(withEmailVerified(false), withPasswordHash(strPtr(string(hash))))

	mockUsers := &MockUsersRepo{}
	mockUsers.On("GetUserByEmail", mock.Anything, "jane@example.com").Return(user, nil)

	svcCtx := &svc.ServiceContext{
		Repo: &repository.Repository{Users: mockUsers},
	}

	l := NewLoginLogic(ctx, svcCtx)
	_, err = l.Login(&auth.LoginRequest{
		Email:    "jane@example.com",
		Password: "Abcdef1!",
	})

	assertGrpcError(t, err, codes.PermissionDenied, MsgEmailNotVerified)
}

func TestLoginLogic_AccessTokenCreationFails(t *testing.T) {
	ctx := context.Background()
	hash, err := bcrypt.GenerateFromPassword([]byte("Abcdef1!"), bcrypt.MinCost)
	require.NoError(t, err)
	user := makeTestUser(withPasswordHash(strPtr(string(hash))))

	mockUsers := &MockUsersRepo{}
	mockTokenMaker := &MockTokenMaker{}

	mockUsers.On("GetUserByEmail", mock.Anything, "jane@example.com").Return(user, nil)
	mockTokenMaker.On("CreateAccessToken", mock.Anything, user.ID, "janedoe", []string{"user"}, mock.AnythingOfType("uuid.UUID")).
		Return(nil, errors.New("signing failed"))

	svcCtx := &svc.ServiceContext{
		Repo:       &repository.Repository{Users: mockUsers},
		TokenMaker: mockTokenMaker,
		Config:     testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewLoginLogic(ctx, svcCtx)
	_, err = l.Login(&auth.LoginRequest{
		Email:    "jane@example.com",
		Password: "Abcdef1!",
	})

	assertGrpcError(t, err, codes.Internal, MsgFailedGenerateAccessToken)
}

func TestLoginLogic_RefreshTokenCreationFails(t *testing.T) {
	ctx := context.Background()
	hash, err := bcrypt.GenerateFromPassword([]byte("Abcdef1!"), bcrypt.MinCost)
	require.NoError(t, err)
	user := makeTestUser(withPasswordHash(strPtr(string(hash))))

	mockUsers := &MockUsersRepo{}
	mockTokenMaker := &MockTokenMaker{}

	mockUsers.On("GetUserByEmail", mock.Anything, "jane@example.com").Return(user, nil)
	mockTokenMaker.On("CreateAccessToken", mock.Anything, user.ID, "janedoe", []string{"user"}, mock.AnythingOfType("uuid.UUID")).
		Return(&jwt.TokenResponse{Token: "access-token"}, nil)
	mockTokenMaker.On("CreateRefreshToken", mock.Anything, user.ID, "janedoe", []string{"user"}, mock.AnythingOfType("uuid.UUID")).
		Return(nil, errors.New("signing failed"))

	svcCtx := &svc.ServiceContext{
		Repo:       &repository.Repository{Users: mockUsers},
		TokenMaker: mockTokenMaker,
		Config:     testConfig(15*time.Minute, 7*24*time.Hour),
	}

	l := NewLoginLogic(ctx, svcCtx)
	_, err = l.Login(&auth.LoginRequest{
		Email:    "jane@example.com",
		Password: "Abcdef1!",
	})

	assertGrpcError(t, err, codes.Internal, MsgFailedGenerateRefreshToken)
}
