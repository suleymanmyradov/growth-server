package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// UserSettingsRepo implements IUserSettings interface
type UserSettingsRepo struct {
	db *db.Queries
}

// NewUserSettingsRepo creates a new UserSettingsRepo instance
func NewUserSettingsRepo(db *db.Queries) *UserSettingsRepo {
	return &UserSettingsRepo{db: db}
}

func (r *UserSettingsRepo) GetUserSettings(ctx context.Context, userID uuid.UUID) (db.UserSetting, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserSettingsRepo.GetUserSettings")
	defer span.End()

	return r.db.GetUserSettings(ctx, userID)
}

func (r *UserSettingsRepo) GetUserSettingsByID(ctx context.Context, id uuid.UUID) (db.UserSetting, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserSettingsRepo.GetUserSettingsByID")
	defer span.End()

	return r.db.GetUserSettingsByID(ctx, id)
}

func (r *UserSettingsRepo) CreateUserSettings(ctx context.Context, params db.CreateUserSettingsParams) (db.UserSetting, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserSettingsRepo.CreateUserSettings")
	defer span.End()

	return r.db.CreateUserSettings(ctx, params)
}

func (r *UserSettingsRepo) UpdateUserSettings(ctx context.Context, params db.UpdateUserSettingsParams) (db.UserSetting, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserSettingsRepo.UpdateUserSettings")
	defer span.End()

	return r.db.UpdateUserSettings(ctx, params)
}

func (r *UserSettingsRepo) DeleteUserSettings(ctx context.Context, userID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserSettingsRepo.DeleteUserSettings")
	defer span.End()

	return r.db.DeleteUserSettings(ctx, userID)
}

func (r *UserSettingsRepo) CountUserSettings(ctx context.Context) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "UserSettingsRepo.CountUserSettings")
	defer span.End()

	return r.db.CountUserSettings(ctx)
}
