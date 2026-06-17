package repository

import (
	"context"
	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
)

type IInternalUsers interface {
	Create(ctx context.Context, email, passwordHash, fullName, role string) (db.InternalUser, error)
	GetByEmail(ctx context.Context, email string) (db.InternalUser, error)
	GetByID(ctx context.Context, id uuid.UUID) (db.InternalUser, error)
	UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) (db.InternalUser, error)
	UpdateProfile(ctx context.Context, id uuid.UUID, fullName string) (db.InternalUser, error)
}

type Repository struct {
	InternalUsers IInternalUsers
}

func NewRepository(dbq *db.Queries) *Repository {
	return &Repository{
		InternalUsers: NewInternalUsersRepo(dbq),
	}
}
