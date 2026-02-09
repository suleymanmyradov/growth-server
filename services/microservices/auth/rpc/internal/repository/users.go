package repository

import (
    "context"

    "github.com/google/uuid"
    "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
    "github.com/zeromicro/go-zero/core/trace"
)

// UsersRepo implements IUsers interface
type UsersRepo struct {
    db *db.Queries
}

// NewUsersRepo creates a new UsersRepo instance
func NewUsersRepo(dbq *db.Queries) *UsersRepo {
    return &UsersRepo{db: dbq}
}

func (r *UsersRepo) CreateUser(ctx context.Context, params db.CreateUserParams) (db.User, error) {
    ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UsersRepo.CreateUser")
    defer span.End()

    return r.db.CreateUser(ctx, params)
}

func (r *UsersRepo) GetUserByEmail(ctx context.Context, email string) (db.User, error) {
    ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UsersRepo.GetUserByEmail")
    defer span.End()

    return r.db.GetUserByEmail(ctx, email)
}

func (r *UsersRepo) GetUserByID(ctx context.Context, id uuid.UUID) (db.User, error) {
    ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UsersRepo.GetUserByID")
    defer span.End()

    return r.db.GetUserByID(ctx, id)
}

func (r *UsersRepo) GetUserByUsername(ctx context.Context, username string) (db.User, error) {
    ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UsersRepo.GetUserByUsername")
    defer span.End()

    return r.db.GetUserByUsername(ctx, username)
}

func (r *UsersRepo) UpdateUserPassword(ctx context.Context, params db.UpdateUserPasswordParams) (db.User, error) {
    ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UsersRepo.UpdateUserPassword")
    defer span.End()

    return r.db.UpdateUserPassword(ctx, params)
}

func (r *UsersRepo) UpdateUserFullName(ctx context.Context, params db.UpdateUserFullNameParams) (db.User, error) {
    ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UsersRepo.UpdateUserFullName")
    defer span.End()

    return r.db.UpdateUserFullName(ctx, params)
}
