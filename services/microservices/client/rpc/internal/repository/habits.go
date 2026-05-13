package repository

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

type habitsRepo struct {
	db *db.Queries
}

func NewHabitsRepo(queries *db.Queries) IHabits {
	return &habitsRepo{db: queries}
}

func (r *habitsRepo) ListHabits(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Habit, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.ListHabits")
	defer span.End()

	return r.db.ListHabits(ctx, db.ListHabitsParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *habitsRepo) GetHabitByID(ctx context.Context, id uuid.UUID) (db.Habit, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.GetHabitByID")
	defer span.End()

	return r.db.GetHabit(ctx, id)
}

func (r *habitsRepo) CreateHabit(ctx context.Context, params db.CreateHabitParams) (db.Habit, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.CreateHabit")
	defer span.End()

	return r.db.CreateHabit(ctx, params)
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

func (r *habitsRepo) ToggleHabit(ctx context.Context, id uuid.UUID) (db.Habit, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.ToggleHabit")
	defer span.End()

	return r.db.ToggleHabit(ctx, id)
}

func (r *habitsRepo) UpdateHabitStreak(ctx context.Context, id uuid.UUID, streak int32) (db.Habit, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitsRepo.UpdateHabitStreak")
	defer span.End()

	return r.db.UpdateHabitStreak(ctx, db.UpdateHabitStreakParams{
		ID:     id,
		Streak: sql.NullInt32{Int32: streak, Valid: true},
	})
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
