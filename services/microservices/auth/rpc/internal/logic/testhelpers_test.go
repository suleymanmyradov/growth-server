package logic

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/mock"
	"github.com/suleymanmyradov/growth-server/pkg/auth/jwt"
	"github.com/suleymanmyradov/growth-server/pkg/email"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
)

// ============================================
// MockTokenMaker
// ============================================

type MockTokenMaker struct {
	mock.Mock
}

func (m *MockTokenMaker) CreateAccessToken(ctx context.Context, userID uuid.UUID, username string, roles []string, sessionID uuid.UUID) (*jwt.TokenResponse, error) {
	args := m.Called(ctx, userID, username, roles, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwt.TokenResponse), args.Error(1)
}

func (m *MockTokenMaker) CreateRefreshToken(ctx context.Context, userID uuid.UUID, username string, roles []string, sessionID uuid.UUID) (*jwt.TokenResponse, error) {
	args := m.Called(ctx, userID, username, roles, sessionID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwt.TokenResponse), args.Error(1)
}

func (m *MockTokenMaker) VerifyAccessToken(ctx context.Context, tokenString string) (*jwt.TokenClaims, error) {
	args := m.Called(ctx, tokenString)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwt.TokenClaims), args.Error(1)
}

func (m *MockTokenMaker) VerifyRefreshToken(ctx context.Context, tokenString string) (*jwt.TokenClaims, error) {
	args := m.Called(ctx, tokenString)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwt.TokenClaims), args.Error(1)
}

func (m *MockTokenMaker) RevokeAccessToken(ctx context.Context, tokenString string) error {
	return m.Called(ctx, tokenString).Error(0)
}

func (m *MockTokenMaker) RevokeRefreshToken(ctx context.Context, tokenString string) error {
	return m.Called(ctx, tokenString).Error(0)
}

func (m *MockTokenMaker) RevokeSession(ctx context.Context, sessionID uuid.UUID, ttl time.Duration) error {
	return m.Called(ctx, sessionID, ttl).Error(0)
}

func (m *MockTokenMaker) IsSessionRevoked(ctx context.Context, sessionID uuid.UUID) (bool, error) {
	args := m.Called(ctx, sessionID)
	return args.Bool(0), args.Error(1)
}

func (m *MockTokenMaker) RotateRefreshToken(ctx context.Context, oldToken string) (*jwt.TokenResponse, error) {
	args := m.Called(ctx, oldToken)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*jwt.TokenResponse), args.Error(1)
}

// ============================================
// MockTxRunner
// ============================================

type MockTxRunner struct {
	mock.Mock
}

func (m *MockTxRunner) Run(ctx context.Context, userID string, fn func(pgx.Tx) error) error {
	// By default, do NOT call fn — the register logic creates db.New(tx) inside
	// the callback, which would panic with a nil tx. Tests that need to simulate
	// a specific return value just configure the mock to return that error.
	return m.Called(ctx, userID, fn).Error(0)
}

// ============================================
// MockUsersRepo
// ============================================

type MockUsersRepo struct {
	mock.Mock
}

func (m *MockUsersRepo) CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error) {
	args := m.Called(ctx, arg)
	if args.Get(0) == nil {
		return db.User{}, args.Error(1)
	}
	return args.Get(0).(db.User), args.Error(1)
}

func (m *MockUsersRepo) CreateUserOAuth(ctx context.Context, username, email, fullName string, emailVerified bool) (db.User, error) {
	args := m.Called(ctx, username, email, fullName, emailVerified)
	if args.Get(0) == nil {
		return db.User{}, args.Error(1)
	}
	return args.Get(0).(db.User), args.Error(1)
}

func (m *MockUsersRepo) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
	args := m.Called(ctx, email)
	if args.Get(0) == nil {
		return db.User{}, args.Error(1)
	}
	return args.Get(0).(db.User), args.Error(1)
}

func (m *MockUsersRepo) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return db.User{}, args.Error(1)
	}
	return args.Get(0).(db.User), args.Error(1)
}

func (m *MockUsersRepo) GetUserByUsername(ctx context.Context, username string) (db.User, error) {
	args := m.Called(ctx, username)
	if args.Get(0) == nil {
		return db.User{}, args.Error(1)
	}
	return args.Get(0).(db.User), args.Error(1)
}

func (m *MockUsersRepo) UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordHash string) (db.User, error) {
	args := m.Called(ctx, id, passwordHash)
	if args.Get(0) == nil {
		return db.User{}, args.Error(1)
	}
	return args.Get(0).(db.User), args.Error(1)
}

func (m *MockUsersRepo) UpdateUserFullName(ctx context.Context, id uuid.UUID, fullName string) (db.User, error) {
	args := m.Called(ctx, id, fullName)
	if args.Get(0) == nil {
		return db.User{}, args.Error(1)
	}
	return args.Get(0).(db.User), args.Error(1)
}

func (m *MockUsersRepo) SetEmailVerified(ctx context.Context, id uuid.UUID) (db.User, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return db.User{}, args.Error(1)
	}
	return args.Get(0).(db.User), args.Error(1)
}

func (m *MockUsersRepo) UpdateUserProfile(ctx context.Context, params db.UpdateUserProfileParams) (db.User, error) {
	args := m.Called(ctx, params)
	if args.Get(0) == nil {
		return db.User{}, args.Error(1)
	}
	return args.Get(0).(db.User), args.Error(1)
}

func (m *MockUsersRepo) DeleteUser(ctx context.Context, id uuid.UUID) error {
	return m.Called(ctx, id).Error(0)
}

func (m *MockUsersRepo) ListUserIds(ctx context.Context, cursor uuid.UUID, limit int32) ([]uuid.UUID, error) {
	args := m.Called(ctx, cursor, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]uuid.UUID), args.Error(1)
}

// ============================================
// MockEmailSender
// ============================================

type MockEmailSender struct {
	mock.Mock
	mu   sync.Mutex
	Sent []email.Email
}

func (m *MockEmailSender) Send(ctx context.Context, msg email.Email) error {
	m.mu.Lock()
	m.Sent = append(m.Sent, msg)
	m.mu.Unlock()
	return m.Called(ctx, msg).Error(0)
}

// ============================================
// Test helpers
// ============================================

// makeTestUser creates a db.User with sensible defaults for tests.
func makeTestUser(overrides ...func(*db.User)) db.User {
	hash := "hashed-password"
	now := time.Now().UTC()
	u := db.User{
		ID:            uuid.New(),
		Username:      "janedoe",
		Email:         "jane@example.com",
		PasswordHash:  &hash,
		FullName:      "Jane Doe",
		Bio:           nil,
		Location:      nil,
		Website:       nil,
		Interests:     []string{},
		AvatarUrl:     nil,
		CreatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
		UpdatedAt:     pgtype.Timestamptz{Time: now, Valid: true},
		EmailVerified: true,
	}
	for _, fn := range overrides {
		fn(&u)
	}
	return u
}

// withEmailVerified sets EmailVerified on the user.
func withEmailVerified(verified bool) func(*db.User) {
	return func(u *db.User) { u.EmailVerified = verified }
}

// withPasswordHash sets PasswordHash on the user.
func withPasswordHash(hash *string) func(*db.User) {
	return func(u *db.User) { u.PasswordHash = hash }
}

// testConfig returns a config.Config with JWT expiry durations set.
// Uses a helper because the JWT field is an anonymous struct with tags,
// making inline literals verbose in tests.
func testConfig(accessExpiry, refreshExpiry time.Duration) config.Config {
	cfg := config.Config{}
	cfg.JWT.AccessExpiryDuration = accessExpiry
	cfg.JWT.RefreshExpiryDuration = refreshExpiry
	cfg.Email.FrontendBaseURL = "http://localhost:3000"
	return cfg
}
