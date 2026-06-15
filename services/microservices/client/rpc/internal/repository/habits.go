package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

type habitsRepo struct {
	db *db.Queries
}

func NewHabitsRepo(queries *db.Queries) IHabits {
	return &habitsRepo{db: queries}
}

// WithTx returns a new habitsRepo backed by the given transaction.
func (r *habitsRepo) WithTx(tx pgx.Tx) *habitsRepo {
	return &habitsRepo{db: r.db.WithTx(tx)}
}

func (r *habitsRepo) ListHabits(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.GetHabitRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.ListHabits")
	defer span.End()

	rows, err := r.db.ListHabits(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]db.GetHabitRow, len(rows))
	for i, row := range rows {
		out[i] = db.GetHabitRow(row)
	}
	return out, nil
}

func (r *habitsRepo) GetHabitByID(ctx context.Context, id uuid.UUID) (db.GetHabitRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.GetHabitByID")
	defer span.End()

	return r.db.GetHabit(ctx, id)
}

func (r *habitsRepo) CreateHabit(ctx context.Context, name string, description *string, category string, userID uuid.UUID) (db.GetHabitRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.CreateHabit")
	defer span.End()

	row, err := r.db.CreateHabit(ctx, name, description, category, userID)
	return db.GetHabitRow(row), err
}

func (r *habitsRepo) UpdateHabit(ctx context.Context, id uuid.UUID, name string, description *string, category string) (db.GetHabitRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.UpdateHabit")
	defer span.End()

	row, err := r.db.UpdateHabit(ctx, id, name, description, category)
	return db.GetHabitRow(row), err
}

func (r *habitsRepo) DeleteHabit(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.DeleteHabit")
	defer span.End()

	return r.db.DeleteHabit(ctx, id)
}

func (r *habitsRepo) ToggleHabit(ctx context.Context, id uuid.UUID) (db.GetHabitRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.ToggleHabit")
	defer span.End()

	row, err := r.db.ToggleHabit(ctx, id)
	return db.GetHabitRow(row), err
}

func (r *habitsRepo) UpdateHabitStreak(ctx context.Context, id uuid.UUID, streak int32) (db.GetHabitRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.UpdateHabitStreak")
	defer span.End()

	row, err := r.db.UpdateHabitStreak(ctx, id, streak)
	return db.GetHabitRow(row), err
}

func (r *habitsRepo) MarkHabitCompleted(ctx context.Context, id uuid.UUID) (db.GetHabitRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.MarkHabitCompleted")
	defer span.End()

	row, err := r.db.MarkHabitCompleted(ctx, id)
	return db.GetHabitRow(row), err
}

func (r *habitsRepo) ResetTodayHabits(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.ResetTodayHabits")
	defer span.End()

	return r.db.ResetTodayHabits(ctx, userID)
}

func (r *habitsRepo) CountHabitsByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.CountHabitsByUser")
	defer span.End()

	return r.db.CountHabitsByUser(ctx, userID)
}
