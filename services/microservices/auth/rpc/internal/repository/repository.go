package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
)

// IUsers defines the interface for user repository operations
type IUsers interface {
	CreateUser(ctx context.Context, arg db.CreateUserParams) (db.User, error)
	CreateUserOAuth(ctx context.Context, username, email, fullName string, emailVerified bool) (db.User, error)
	GetUserByEmail(ctx context.Context, email string) (db.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	GetUserByUsername(ctx context.Context, username string) (db.User, error)
	UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordHash string) (db.User, error)
	UpdateUserFullName(ctx context.Context, id uuid.UUID, fullName string) (db.User, error)
	SetEmailVerified(ctx context.Context, id uuid.UUID) (db.User, error)
	UpdateUserProfile(ctx context.Context, params db.UpdateUserProfileParams) (db.User, error)
}

// IOauth defines OAuth account linking operations.
type IOauth interface {
	GetOAuthAccount(ctx context.Context, provider, providerUID string) (db.UserOauthAccount, error)
	CreateOAuthAccount(ctx context.Context, userID uuid.UUID, provider, providerUID string, email *string) (db.UserOauthAccount, error)
}

// Repository aggregates all auth repositories
type Repository struct {
	Users IUsers
	Oauth IOauth
}

// NewRepository creates a new Repository instance with all implementations
func NewRepository(dbq *db.Queries) *Repository {
	return &Repository{
		Users: NewUsersRepo(dbq),
		Oauth: NewOauthRepo(dbq),
	}
}
