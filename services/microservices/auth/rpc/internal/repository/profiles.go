package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// ProfilesRepo implements IProfiles interface
type ProfilesRepo struct {
	db *db.Queries
}

// NewProfilesRepo creates a new ProfilesRepo instance
func NewProfilesRepo(dbq *db.Queries) *ProfilesRepo {
	return &ProfilesRepo{db: dbq}
}

// WithTx returns a new ProfilesRepo backed by the given transaction.
func (r *ProfilesRepo) WithTx(tx pgx.Tx) *ProfilesRepo {
	return &ProfilesRepo{db: r.db.WithTx(tx)}
}

func (r *ProfilesRepo) CreateProfile(ctx context.Context, params db.CreateProfileParams) (db.Profile, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ProfilesRepo.CreateProfile")
	defer span.End()

	return r.db.CreateProfile(ctx, params)
}

func (r *ProfilesRepo) GetProfileByUserID(ctx context.Context, userID uuid.UUID) (db.Profile, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ProfilesRepo.GetProfileByUserID")
	defer span.End()

	return r.db.GetProfileByUserID(ctx, userID)
}

func (r *ProfilesRepo) UpdateProfile(ctx context.Context, params db.UpdateProfileParams) (db.Profile, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ProfilesRepo.UpdateProfile")
	defer span.End()

	return r.db.UpdateProfile(ctx, params)
}
