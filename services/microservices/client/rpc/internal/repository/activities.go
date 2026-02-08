package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// ActivitiesRepo implements IActivities interface
type ActivitiesRepo struct {
	db *db.Queries
}

// NewActivitiesRepo creates a new ActivitiesRepo instance
func NewActivitiesRepo(db *db.Queries) *ActivitiesRepo {
	return &ActivitiesRepo{db: db}
}

func (r *ActivitiesRepo) ListActivities(ctx context.Context, limit, offset int32) ([]db.Activity, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.ListActivities")
	defer span.End()

	return r.db.ListActivities(ctx, db.ListActivitiesParams{
		Limit:  limit,
		Offset: offset,
	})
}

func (r *ActivitiesRepo) ListActivitiesByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Activity, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.ListActivitiesByUser")
	defer span.End()

	return r.db.ListActivitiesByUser(ctx, db.ListActivitiesByUserParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *ActivitiesRepo) ListActivitiesByType(ctx context.Context, userID uuid.UUID, itemType string, limit, offset int32) ([]db.Activity, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.ListActivitiesByType")
	defer span.End()

	return r.db.ListActivitiesByType(ctx, db.ListActivitiesByTypeParams{
		UserID:   userID,
		ItemType: itemType,
		Limit:    limit,
		Offset:   offset,
	})
}

func (r *ActivitiesRepo) GetActivity(ctx context.Context, id uuid.UUID) (db.Activity, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.GetActivity")
	defer span.End()

	return r.db.GetActivity(ctx, id)
}

func (r *ActivitiesRepo) CreateActivity(ctx context.Context, params db.CreateActivityParams) (db.Activity, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.CreateActivity")
	defer span.End()

	return r.db.CreateActivity(ctx, params)
}

func (r *ActivitiesRepo) LogActivity(ctx context.Context, params db.LogActivityParams) (db.Activity, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.LogActivity")
	defer span.End()

	return r.db.LogActivity(ctx, params)
}

func (r *ActivitiesRepo) DeleteActivity(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.DeleteActivity")
	defer span.End()

	return r.db.DeleteActivity(ctx, id)
}

func (r *ActivitiesRepo) DeleteActivitiesByUser(ctx context.Context, userID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.DeleteActivitiesByUser")
	defer span.End()

	return r.db.DeleteActivitiesByUser(ctx, userID)
}

func (r *ActivitiesRepo) CountActivities(ctx context.Context) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.CountActivities")
	defer span.End()

	return r.db.CountActivities(ctx)
}

func (r *ActivitiesRepo) CountActivitiesByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.CountActivitiesByUser")
	defer span.End()

	return r.db.CountActivitiesByUser(ctx, userID)
}

func (r *ActivitiesRepo) CountActivitiesByUserAndType(ctx context.Context, userID uuid.UUID, itemType string) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.CountActivitiesByUserAndType")
	defer span.End()

	return r.db.CountActivitiesByUserAndType(ctx, db.CountActivitiesByUserAndTypeParams{
		UserID:   userID,
		ItemType: itemType,
	})
}

func (r *ActivitiesRepo) GetActivityFeed(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Activity, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.GetActivityFeed")
	defer span.End()

	return r.db.GetActivityFeed(ctx, db.GetActivityFeedParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *ActivitiesRepo) GetActivityStats(ctx context.Context, userID uuid.UUID) (db.GetActivityStatsRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.GetActivityStats")
	defer span.End()

	return r.db.GetActivityStats(ctx, userID)
}

func (r *ActivitiesRepo) GetStreaks(ctx context.Context, userID uuid.UUID) (db.GetStreaksRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.GetStreaks")
	defer span.End()

	return r.db.GetStreaks(ctx, userID)
}

func (r *ActivitiesRepo) GetAchievements(ctx context.Context, userID uuid.UUID) ([]db.GetAchievementsRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.GetAchievements")
	defer span.End()

	return r.db.GetAchievements(ctx, userID)
}

func (r *ActivitiesRepo) GetActivityCalendar(ctx context.Context, userID uuid.UUID, year, month int32) ([]db.GetActivityCalendarRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ActivitiesRepo.GetActivityCalendar")
	defer span.End()

	if year == 0 || month == 0 {
		now := time.Now().UTC()
		year = int32(now.Year())
		month = int32(now.Month())
	}

	start := time.Date(int(year), time.Month(month), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)

	return r.db.GetActivityCalendar(ctx, db.GetActivityCalendarParams{
		UserID:      userID,
		CreatedAt:   sql.NullTime{Time: start, Valid: true},
		CreatedAt_2: sql.NullTime{Time: end, Valid: true},
	})
}
