package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

type weeklyReviewsRepo struct {
	db *db.Queries
}

func NewWeeklyReviewsRepo(queries *db.Queries) IWeeklyReviews {
	return &weeklyReviewsRepo{db: queries}
}

// WithTx returns a new weeklyReviewsRepo backed by the given transaction.
func (r *weeklyReviewsRepo) WithTx(tx pgx.Tx) *weeklyReviewsRepo {
	return &weeklyReviewsRepo{db: r.db.WithTx(tx)}
}

func (r *weeklyReviewsRepo) CreateWeeklyReview(ctx context.Context, params db.CreateWeeklyReviewParams) (db.GetWeeklyReviewRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.CreateWeeklyReview")
	defer span.End()

	row, err := r.db.CreateWeeklyReview(ctx, params)
	return db.GetWeeklyReviewRow(row), err
}

func (r *weeklyReviewsRepo) GetWeeklyReview(ctx context.Context, userID uuid.UUID, weekStart time.Time) (db.GetWeeklyReviewRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.GetWeeklyReview")
	defer span.End()

	return r.db.GetWeeklyReview(ctx, userID, pgtype.Date{Time: weekStart, Valid: true})
}

func (r *weeklyReviewsRepo) GetCurrentWeeklyReview(ctx context.Context, userID uuid.UUID) (db.GetWeeklyReviewRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.GetCurrentWeeklyReview")
	defer span.End()

	row, err := r.db.GetCurrentWeeklyReview(ctx, userID)
	return db.GetWeeklyReviewRow(row), err
}

func (r *weeklyReviewsRepo) ListWeeklyReviews(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.GetWeeklyReviewRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.ListWeeklyReviews")
	defer span.End()

	rows, err := r.db.ListWeeklyReviews(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]db.GetWeeklyReviewRow, len(rows))
	for i, row := range rows {
		out[i] = db.GetWeeklyReviewRow(row)
	}
	return out, nil
}

func (r *weeklyReviewsRepo) CountWeeklyReviews(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.CountWeeklyReviews")
	defer span.End()

	return r.db.CountWeeklyReviews(ctx, userID)
}

func (r *weeklyReviewsRepo) GetCheckInStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetCheckInStatsForWeekRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.GetCheckInStatsForWeek")
	defer span.End()

	return r.db.GetCheckInStatsForWeek(ctx, userID, pgtype.Timestamptz{Time: start, Valid: true}, pgtype.Timestamptz{Time: end, Valid: true})
}

func (r *weeklyReviewsRepo) GetDailyCheckInStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetDailyCheckInStatsForWeekRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.GetDailyCheckInStatsForWeek")
	defer span.End()

	return r.db.GetDailyCheckInStatsForWeek(ctx, userID, pgtype.Timestamptz{Time: start, Valid: true}, pgtype.Timestamptz{Time: end, Valid: true})
}

func (r *weeklyReviewsRepo) GetBlockerStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetBlockerStatsForWeekRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.GetBlockerStatsForWeek")
	defer span.End()

	return r.db.GetBlockerStatsForWeek(ctx, userID, pgtype.Timestamptz{Time: start, Valid: true}, pgtype.Timestamptz{Time: end, Valid: true})
}

func (r *weeklyReviewsRepo) GetMoodStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetMoodStatsForWeekRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.GetMoodStatsForWeek")
	defer span.End()

	return r.db.GetMoodStatsForWeek(ctx, userID, pgtype.Timestamptz{Time: start, Valid: true}, pgtype.Timestamptz{Time: end, Valid: true})
}

func (r *weeklyReviewsRepo) GetEnergyStatsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.GetEnergyStatsForWeekRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "WeeklyReviewsRepo.GetEnergyStatsForWeek")
	defer span.End()

	return r.db.GetEnergyStatsForWeek(ctx, userID, pgtype.Timestamptz{Time: start, Valid: true}, pgtype.Timestamptz{Time: end, Valid: true})
}
