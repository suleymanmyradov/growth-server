package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
)

// IUsers defines the interface for user repository operations
type IUsers interface {
	CreateUser(ctx context.Context, username string, email string, passwordHash string, fullName string) (db.User, error)
	GetUserByEmail(ctx context.Context, email string) (db.User, error)
	GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error)
	GetUserByUsername(ctx context.Context, username string) (db.User, error)
	UpdateUserPassword(ctx context.Context, id uuid.UUID, passwordHash string) (db.User, error)
	UpdateUserFullName(ctx context.Context, id uuid.UUID, fullName string) (db.User, error)
	UpdateUserProfile(ctx context.Context, params db.UpdateUserProfileParams) (db.User, error)
}

// Repository aggregates all auth repositories
type Repository struct {
	Users IUsers
}

// NewRepository creates a new Repository instance with all implementations
func NewRepository(dbq *db.Queries) *Repository {
	return &Repository{
		Users: NewUsersRepo(dbq),
	}
}
