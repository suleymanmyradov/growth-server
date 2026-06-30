package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// UsersRepo implements IUsers interface.
type UsersRepo struct {
	db *db.Queries
}

// NewUsersRepo creates a new UsersRepo instance.
func NewUsersRepo(dbq *db.Queries) *UsersRepo {
	return &UsersRepo{db: dbq}
}

// WithTx returns a new UsersRepo backed by the given transaction.
func (r *UsersRepo) WithTx(tx pgx.Tx) *UsersRepo {
	return &UsersRepo{db: r.db.WithTx(tx)}
}

func (r *UsersRepo) GetUserProfileByID(ctx context.Context, id uuid.UUID) (db.GetUserProfileByIDRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UsersRepo.GetUserProfileByID")
	defer span.End()
	return r.db.GetUserProfileByID(ctx, id)
}
