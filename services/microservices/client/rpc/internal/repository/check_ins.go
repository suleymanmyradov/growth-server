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

type checkInsRepo struct {
	db *db.Queries
}

func NewCheckInsRepo(queries *db.Queries) ICheckIns {
	return &checkInsRepo{db: queries}
}

// WithTx returns a new checkInsRepo backed by the given transaction.
func (r *checkInsRepo) WithTx(tx pgx.Tx) *checkInsRepo {
	return &checkInsRepo{db: r.db.WithTx(tx)}
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

func (r *checkInsRepo) GetCheckInsByHabit(ctx context.Context, habitID, userID uuid.UUID, limit, offset int32) ([]db.CheckIn, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CheckInsRepo.GetCheckInsByHabit")
	defer span.End()

	return r.db.GetCheckInsByHabit(ctx, habitID, userID, limit, offset)
}

func (r *checkInsRepo) GetCheckInsByUser(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.CheckIn, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CheckInsRepo.GetCheckInsByUser")
	defer span.End()

	return r.db.GetCheckInsByUser(ctx, userID, limit, offset)
}

func (r *checkInsRepo) GetCheckInHistory(ctx context.Context, userID uuid.UUID, start, end time.Time, limit, offset int32) ([]db.CheckIn, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CheckInsRepo.GetCheckInHistory")
	defer span.End()

	return r.db.GetCheckInHistory(ctx, db.GetCheckInHistoryParams{
		UserID:      userID,
		CreatedAt:   pgtype.Timestamptz{Time: start, Valid: true},
		CreatedAt_2: pgtype.Timestamptz{Time: end, Valid: true},
		Limit:       limit,
		Offset:      offset,
	})
}

func (r *checkInsRepo) GetCheckInsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.CheckIn, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CheckInsRepo.GetCheckInsForWeek")
	defer span.End()

	return r.db.GetCheckInsForWeek(ctx, userID, pgtype.Timestamptz{Time: start, Valid: true}, pgtype.Timestamptz{Time: end, Valid: true})
}

func (r *checkInsRepo) HasCheckedInToday(ctx context.Context, userID, habitID uuid.UUID) (bool, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CheckInsRepo.HasCheckedInToday")
	defer span.End()

	return r.db.HasCheckedInToday(ctx, userID, habitID)
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
