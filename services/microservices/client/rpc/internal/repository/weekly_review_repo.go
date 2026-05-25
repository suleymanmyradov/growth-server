package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

type weeklyReviewsRepo struct {
	db *db.Queries
}

func NewWeeklyReviewsRepo(queries *db.Queries) IWeeklyReviews {
	return &weeklyReviewsRepo{db: queries}
}

func (r *weeklyReviewsRepo) CreateWeeklyReview(ctx context.Context, params db.CreateWeeklyReviewParams) (db.WeeklyReview, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.CreateWeeklyReview")
	defer span.End()

	return r.db.CreateWeeklyReview(ctx, params)
}

func (r *weeklyReviewsRepo) GetWeeklyReview(ctx context.Context, userID uuid.UUID, weekStart time.Time) (db.WeeklyReview, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.GetWeeklyReview")
	defer span.End()

	return r.db.GetWeeklyReview(ctx, db.GetWeeklyReviewParams{
		UserID:    userID,
		WeekStart: weekStart,
	})
}

func (r *weeklyReviewsRepo) GetCurrentWeeklyReview(ctx context.Context, userID uuid.UUID) (db.WeeklyReview, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.GetCurrentWeeklyReview")
	defer span.End()

	return r.db.GetCurrentWeeklyReview(ctx, userID)
}

func (r *weeklyReviewsRepo) ListWeeklyReviews(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.WeeklyReview, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.ListWeeklyReviews")
	defer span.End()

	return r.db.ListWeeklyReviews(ctx, db.ListWeeklyReviewsParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *weeklyReviewsRepo) CountWeeklyReviews(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.CountWeeklyReviews")
	defer span.End()

	return r.db.CountWeeklyReviews(ctx, userID)
}

func (r *weeklyReviewsRepo) GetCheckInStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetCheckInStatsForWeekRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.GetCheckInStatsForWeek")
	defer span.End()

	return r.db.GetCheckInStatsForWeek(ctx, db.GetCheckInStatsForWeekParams{
		UserID:      userID,
		CreatedAt:   start,
		CreatedAt_2: end,
	})
}

func (r *weeklyReviewsRepo) GetDailyCheckInStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetDailyCheckInStatsForWeekRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.GetDailyCheckInStatsForWeek")
	defer span.End()

	return r.db.GetDailyCheckInStatsForWeek(ctx, db.GetDailyCheckInStatsForWeekParams{
		UserID:      userID,
		CreatedAt:   start,
		CreatedAt_2: end,
	})
}

func (r *weeklyReviewsRepo) GetBlockerStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetBlockerStatsForWeekRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.GetBlockerStatsForWeek")
	defer span.End()

	return r.db.GetBlockerStatsForWeek(ctx, db.GetBlockerStatsForWeekParams{
		UserID:      userID,
		CreatedAt:   start,
		CreatedAt_2: end,
	})
}

func (r *weeklyReviewsRepo) GetMoodStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetMoodStatsForWeekRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.GetMoodStatsForWeek")
	defer span.End()

	return r.db.GetMoodStatsForWeek(ctx, db.GetMoodStatsForWeekParams{
		UserID:      userID,
		CreatedAt:   start,
		CreatedAt_2: end,
	})
}

func (r *weeklyReviewsRepo) GetEnergyStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetEnergyStatsForWeekRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.GetEnergyStatsForWeek")
	defer span.End()

	return r.db.GetEnergyStatsForWeek(ctx, db.GetEnergyStatsForWeekParams{
		UserID:      userID,
		CreatedAt:   start,
		CreatedAt_2: end,
	})
}
