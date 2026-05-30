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

func (r *habitsRepo) ListHabits(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Habit, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.ListHabits")
	defer span.End()

	return r.db.ListHabits(ctx, userID, limit, offset)
}

func (r *habitsRepo) GetHabitByID(ctx context.Context, id uuid.UUID) (db.Habit, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.GetHabitByID")
	defer span.End()

	return r.db.GetHabit(ctx, id)
}

func (r *habitsRepo) CreateHabit(ctx context.Context, name string, description *string, category string, userID uuid.UUID) (db.Habit, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.CreateHabit")
	defer span.End()

	return r.db.CreateHabit(ctx, name, description, category, userID)
}

func (r *habitsRepo) UpdateHabit(ctx context.Context, params db.UpdateHabitParams) (db.Habit, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.UpdateHabit")
	defer span.End()

	return r.db.UpdateHabit(ctx, params)
}

func (r *habitsRepo) DeleteHabit(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.DeleteHabit")
	defer span.End()

	return r.db.DeleteHabit(ctx, id)
}

func (r *habitsRepo) ToggleHabit(ctx context.Context, id uuid.UUID, version int32) (db.Habit, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.ToggleHabit")
	defer span.End()

	return r.db.ToggleHabit(ctx, id, version)
}

func (r *habitsRepo) UpdateHabitStreak(ctx context.Context, id uuid.UUID, streak int32, version int32) (db.Habit, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.UpdateHabitStreak")
	defer span.End()

	return r.db.UpdateHabitStreak(ctx, id, streak, version)
}

func (r *habitsRepo) MarkHabitCompleted(ctx context.Context, id uuid.UUID, version int32) (db.Habit, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.MarkHabitCompleted")
	defer span.End()

	return r.db.MarkHabitCompleted(ctx, id, version)
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
