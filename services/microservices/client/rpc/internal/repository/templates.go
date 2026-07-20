package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// HabitTemplatesRepo implements IHabitTemplates interface
type HabitTemplatesRepo struct {
	db *db.Queries
}

func NewHabitTemplatesRepo(db *db.Queries) *HabitTemplatesRepo {
	return &HabitTemplatesRepo{db: db}
}

func (r *HabitTemplatesRepo) WithTx(tx pgx.Tx) *HabitTemplatesRepo {
	return &HabitTemplatesRepo{db: r.db.WithTx(tx)}
}

func (r *HabitTemplatesRepo) ListHabitTemplates(ctx context.Context) ([]db.ListHabitTemplatesRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitTemplatesRepo.ListHabitTemplates")
	defer span.End()
	return r.db.ListHabitTemplates(ctx)
}

// GoalTemplatesRepo implements IGoalTemplates interface
type GoalTemplatesRepo struct {
	db *db.Queries
}

func NewGoalTemplatesRepo(db *db.Queries) *GoalTemplatesRepo {
	return &GoalTemplatesRepo{db: db}
}

func (r *GoalTemplatesRepo) WithTx(tx pgx.Tx) *GoalTemplatesRepo {
	return &GoalTemplatesRepo{db: r.db.WithTx(tx)}
}

func (r *GoalTemplatesRepo) ListGoalTemplates(ctx context.Context) ([]db.ListGoalTemplatesRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalTemplatesRepo.ListGoalTemplates")
	defer span.End()
	return r.db.ListGoalTemplates(ctx)
}
