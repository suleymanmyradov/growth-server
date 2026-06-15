package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

type goalsRepo struct {
	db *db.Queries
}

func NewGoalsRepo(queries *db.Queries) IGoals {
	return &goalsRepo{db: queries}
}

// WithTx returns a new goalsRepo backed by the given transaction.
func (r *goalsRepo) WithTx(tx pgx.Tx) *goalsRepo {
	return &goalsRepo{db: r.db.WithTx(tx)}
}

func (r *goalsRepo) ListGoals(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.GetGoalRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.ListGoals")
	defer span.End()

	rows, err := r.db.ListGoals(ctx, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]db.GetGoalRow, len(rows))
	for i, row := range rows {
		out[i] = db.GetGoalRow(row)
	}
	return out, nil
}

func (r *goalsRepo) GetGoalByID(ctx context.Context, id uuid.UUID) (db.GetGoalRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.GetGoalByID")
	defer span.End()

	return r.db.GetGoal(ctx, id)
}

func (r *goalsRepo) CreateGoal(ctx context.Context, params db.CreateGoalParams) (db.GetGoalRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.CreateGoal")
	defer span.End()

	row, err := r.db.CreateGoal(ctx, params)
	return db.GetGoalRow(row), err
}

func (r *goalsRepo) UpdateGoal(ctx context.Context, params db.UpdateGoalParams) (db.GetGoalRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.UpdateGoal")
	defer span.End()

	row, err := r.db.UpdateGoal(ctx, params)
	return db.GetGoalRow(row), err
}

func (r *goalsRepo) DeleteGoal(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.DeleteGoal")
	defer span.End()

	return r.db.DeleteGoal(ctx, id)
}

func (r *goalsRepo) ToggleGoal(ctx context.Context, id uuid.UUID) (db.GetGoalRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.ToggleGoal")
	defer span.End()

	row, err := r.db.ToggleGoal(ctx, id)
	return db.GetGoalRow(row), err
}

func (r *goalsRepo) UpdateGoalProgress(ctx context.Context, id uuid.UUID, progress int32) (db.GetGoalRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.UpdateGoalProgress")
	defer span.End()

	row, err := r.db.UpdateGoalProgress(ctx, id, progress)
	return db.GetGoalRow(row), err
}

func (r *goalsRepo) CountGoalsByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.CountGoalsByUser")
	defer span.End()

	return r.db.CountGoalsByUser(ctx, userID)
}
