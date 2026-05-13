package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

type checkInsRepo struct {
	db *db.Queries
}

func NewCheckInsRepo(queries *db.Queries) ICheckIns {
	return &checkInsRepo{db: queries}
}

func (r *checkInsRepo) CreateCheckIn(ctx context.Context, params db.CreateCheckInParams) (db.CheckIn, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CheckInsRepo.CreateCheckIn")
	defer span.End()

	return r.db.CreateCheckIn(ctx, params)
}

func (r *checkInsRepo) GetTodayCheckIns(ctx context.Context, userID uuid.UUID) ([]db.CheckIn, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CheckInsRepo.GetTodayCheckIns")
	defer span.End()

	return r.db.GetTodayCheckIns(ctx, userID)
}

func (r *checkInsRepo) GetCheckInsByHabit(ctx context.Context, habitID uuid.UUID, limit, offset int32) ([]db.CheckIn, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CheckInsRepo.GetCheckInsByHabit")
	defer span.End()

	return r.db.GetCheckInsByHabit(ctx, db.GetCheckInsByHabitParams{
		HabitID: habitID,
		Limit:   limit,
		Offset:  offset,
	})
}

func (r *checkInsRepo) GetCheckInsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.CheckIn, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CheckInsRepo.GetCheckInsByUser")
	defer span.End()

	return r.db.GetCheckInsByUser(ctx, db.GetCheckInsByUserParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *checkInsRepo) GetCheckInsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.CheckIn, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CheckInsRepo.GetCheckInsForWeek")
	defer span.End()

	return r.db.GetCheckInsForWeek(ctx, db.GetCheckInsForWeekParams{
		UserID:      userID,
		CreatedAt:   start,
		CreatedAt_2: end,
	})
}

func (r *checkInsRepo) HasCheckedInToday(ctx context.Context, userID, habitID uuid.UUID) (bool, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CheckInsRepo.HasCheckedInToday")
	defer span.End()

	return r.db.HasCheckedInToday(ctx, db.HasCheckedInTodayParams{
		UserID:  userID,
		HabitID: habitID,
	})
}

func (r *checkInsRepo) CountCheckInsByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CheckInsRepo.CountCheckInsByUser")
	defer span.End()

	return r.db.CountCheckInsByUser(ctx, userID)
}

func (r *checkInsRepo) CountCheckInsByHabit(ctx context.Context, habitID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CheckInsRepo.CountCheckInsByHabit")
	defer span.End()

	return r.db.CountCheckInsByHabit(ctx, habitID)
}
