package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

type InternalUsersRepo struct {
	db *db.Queries
}

func NewInternalUsersRepo(dbq *db.Queries) *InternalUsersRepo {
	return &InternalUsersRepo{db: dbq}
}

func (r *InternalUsersRepo) Create(ctx context.Context, email, passwordHash, fullName, role string) (db.InternalUser, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "InternalUsersRepo.Create")
	defer span.End()

	return r.db.CreateInternalUser(ctx, email, passwordHash, fullName, role)
}

func (r *InternalUsersRepo) GetByEmail(ctx context.Context, email string) (db.InternalUser, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "InternalUsersRepo.GetByEmail")
	defer span.End()

	return r.db.GetInternalUserByEmail(ctx, email)
}

func (r *InternalUsersRepo) GetByID(ctx context.Context, id uuid.UUID) (db.InternalUser, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "InternalUsersRepo.GetByID")
	defer span.End()

	return r.db.GetInternalUserByID(ctx, id)
}

func (r *InternalUsersRepo) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash string) (db.InternalUser, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "InternalUsersRepo.UpdatePassword")
	defer span.End()

	return r.db.UpdateInternalUserPassword(ctx, id, passwordHash)
}

func (r *InternalUsersRepo) UpdateProfile(ctx context.Context, id uuid.UUID, fullName string) (db.InternalUser, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "InternalUsersRepo.UpdateProfile")
	defer span.End()

	return r.db.UpdateInternalUserProfile(ctx, id, fullName)
}
