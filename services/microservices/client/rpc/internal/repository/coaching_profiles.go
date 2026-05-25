package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// CoachingProfilesRepo implements ICoachingProfiles interface
type CoachingProfilesRepo struct {
	db *db.Queries
}

// NewCoachingProfilesRepo creates a new CoachingProfilesRepo instance
func NewCoachingProfilesRepo(db *db.Queries) *CoachingProfilesRepo {
	return &CoachingProfilesRepo{db: db}
}

func (r *CoachingProfilesRepo) GetCoachingProfile(ctx context.Context, userID uuid.UUID) (db.UserCoachingProfile, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CoachingProfilesRepo.GetCoachingProfile")
	defer span.End()

	return r.db.GetCoachingProfile(ctx, userID)
}

func (r *CoachingProfilesRepo) UpsertCoachingProfile(ctx context.Context, params db.UpsertCoachingProfileParams) (db.UserCoachingProfile, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CoachingProfilesRepo.UpsertCoachingProfile")
	defer span.End()

	return r.db.UpsertCoachingProfile(ctx, params)
}

func (r *CoachingProfilesRepo) UpdateCoachingProfilePreferences(ctx context.Context, params db.UpdateCoachingProfilePreferencesParams) (db.UserCoachingProfile, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CoachingProfilesRepo.UpdateCoachingProfilePreferences")
	defer span.End()

	return r.db.UpdateCoachingProfilePreferences(ctx, params)
}

func (r *CoachingProfilesRepo) UpdateCoachingProfileBlockers(ctx context.Context, params db.UpdateCoachingProfileBlockersParams) (db.UserCoachingProfile, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CoachingProfilesRepo.UpdateCoachingProfileBlockers")
	defer span.End()

	return r.db.UpdateCoachingProfileBlockers(ctx, params)
}

func (r *CoachingProfilesRepo) UpdateCoachingProfileNotes(ctx context.Context, params db.UpdateCoachingProfileNotesParams) (db.UserCoachingProfile, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CoachingProfilesRepo.UpdateCoachingProfileNotes")
	defer span.End()

	return r.db.UpdateCoachingProfileNotes(ctx, params)
}

func (r *CoachingProfilesRepo) UpdateCoachingProfileContextRefresh(ctx context.Context, userID uuid.UUID) (db.UserCoachingProfile, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CoachingProfilesRepo.UpdateCoachingProfileContextRefresh")
	defer span.End()

	return r.db.UpdateCoachingProfileContextRefresh(ctx, userID)
}

func (r *CoachingProfilesRepo) DeleteCoachingProfile(ctx context.Context, userID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CoachingProfilesRepo.DeleteCoachingProfile")
	defer span.End()

	return r.db.DeleteCoachingProfile(ctx, userID)
}
