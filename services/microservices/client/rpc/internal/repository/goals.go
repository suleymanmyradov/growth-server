package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

type goalsRepo struct {
	db *db.Queries
}

func NewGoalsRepo(queries *db.Queries) IGoals {
	return &goalsRepo{db: queries}
}

func (r *goalsRepo) ListGoals(ctx context.Context, userID uuid.UUID, limit, offset int32) ([]db.Goal, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.ListGoals")
	defer span.End()

	return r.db.ListGoals(ctx, db.ListGoalsParams{
		UserID: userID,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *goalsRepo) GetGoalByID(ctx context.Context, id uuid.UUID) (db.Goal, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.GetGoalByID")
	defer span.End()

	return r.db.GetGoal(ctx, id)
}

func (r *goalsRepo) CreateGoal(ctx context.Context, params db.CreateGoalParams) (db.Goal, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.CreateGoal")
	defer span.End()

	return r.db.CreateGoal(ctx, params)
}

func (r *goalsRepo) UpdateGoal(ctx context.Context, params db.UpdateGoalParams) (db.Goal, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.UpdateGoal")
	defer span.End()

	return r.db.UpdateGoal(ctx, params)
}

func (r *goalsRepo) DeleteGoal(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.DeleteGoal")
	defer span.End()

	return r.db.DeleteGoal(ctx, id)
}

func (r *goalsRepo) ToggleGoal(ctx context.Context, id uuid.UUID) (db.Goal, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.ToggleGoal")
	defer span.End()

	return r.db.ToggleGoal(ctx, id)
}

func (r *goalsRepo) UpdateGoalProgress(ctx context.Context, params db.UpdateGoalProgressParams) (db.Goal, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.UpdateGoalProgress")
	defer span.End()

	return r.db.UpdateGoalProgress(ctx, params)
}

func (r *goalsRepo) CountGoalsByUser(ctx context.Context, userID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalsRepo.CountGoalsByUser")
	defer span.End()

	return r.db.CountGoalsByUser(ctx, userID)
}
