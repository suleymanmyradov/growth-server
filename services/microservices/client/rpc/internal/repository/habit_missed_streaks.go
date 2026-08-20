package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

type IHabitMissedStreaks interface {
	IncrementHabitMissedStreak(ctx context.Context, habitID, userID uuid.UUID) (db.HabitMissedStreak, error)
	ResetHabitMissedStreak(ctx context.Context, habitID, userID uuid.UUID, lastCompletedDate pgtype.Date) (db.HabitMissedStreak, error)
	GetHabitMissedStreak(ctx context.Context, habitID uuid.UUID) (db.HabitMissedStreak, error)
	DeleteHabitMissedStreak(ctx context.Context, habitID uuid.UUID) error
}

type habitMissedStreaksRepo struct {
	db *db.Queries
}

func NewHabitMissedStreaksRepo(queries *db.Queries) IHabitMissedStreaks {
	return &habitMissedStreaksRepo{db: queries}
}

func (r *habitMissedStreaksRepo) WithTx(tx pgx.Tx) *habitMissedStreaksRepo {
	return &habitMissedStreaksRepo{db: r.db.WithTx(tx)}
}

func (r *habitMissedStreaksRepo) IncrementHabitMissedStreak(ctx context.Context, habitID, userID uuid.UUID) (db.HabitMissedStreak, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitMissedStreaksRepo.IncrementHabitMissedStreak")
	defer span.End()
	return r.db.IncrementHabitMissedStreak(ctx, habitID, userID)
}

func (r *habitMissedStreaksRepo) ResetHabitMissedStreak(ctx context.Context, habitID, userID uuid.UUID, lastCompletedDate pgtype.Date) (db.HabitMissedStreak, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitMissedStreaksRepo.ResetHabitMissedStreak")
	defer span.End()
	return r.db.ResetHabitMissedStreak(ctx, habitID, userID, lastCompletedDate)
}

func (r *habitMissedStreaksRepo) GetHabitMissedStreak(ctx context.Context, habitID uuid.UUID) (db.HabitMissedStreak, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitMissedStreaksRepo.GetHabitMissedStreak")
	defer span.End()
	return r.db.GetHabitMissedStreak(ctx, habitID)
}

func (r *habitMissedStreaksRepo) DeleteHabitMissedStreak(ctx context.Context, habitID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitMissedStreaksRepo.DeleteHabitMissedStreak")
	defer span.End()
	return r.db.DeleteHabitMissedStreak(ctx, habitID)
}
