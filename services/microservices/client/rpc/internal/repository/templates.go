package repository

import (
	"context"

	"github.com/google/uuid"
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

func (r *HabitTemplatesRepo) AdminListHabitTemplates(ctx context.Context) ([]db.AdminListHabitTemplatesRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitTemplatesRepo.AdminListHabitTemplates")
	defer span.End()
	return r.db.AdminListHabitTemplates(ctx)
}

func (r *HabitTemplatesRepo) AdminGetHabitTemplate(ctx context.Context, id uuid.UUID) (db.AdminGetHabitTemplateRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitTemplatesRepo.AdminGetHabitTemplate")
	defer span.End()
	return r.db.AdminGetHabitTemplate(ctx, id)
}

func (r *HabitTemplatesRepo) AdminCreateHabitTemplate(ctx context.Context, params db.AdminCreateHabitTemplateParams) (db.HabitTemplate, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitTemplatesRepo.AdminCreateHabitTemplate")
	defer span.End()
	return r.db.AdminCreateHabitTemplate(ctx, params)
}

func (r *HabitTemplatesRepo) AdminUpdateHabitTemplate(ctx context.Context, params db.AdminUpdateHabitTemplateParams) (db.HabitTemplate, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitTemplatesRepo.AdminUpdateHabitTemplate")
	defer span.End()
	return r.db.AdminUpdateHabitTemplate(ctx, params)
}

func (r *HabitTemplatesRepo) AdminDeleteHabitTemplate(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "HabitTemplatesRepo.AdminDeleteHabitTemplate")
	defer span.End()
	return r.db.AdminDeleteHabitTemplate(ctx, id)
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

func (r *GoalTemplatesRepo) AdminListGoalTemplates(ctx context.Context) ([]db.AdminListGoalTemplatesRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalTemplatesRepo.AdminListGoalTemplates")
	defer span.End()
	return r.db.AdminListGoalTemplates(ctx)
}

func (r *GoalTemplatesRepo) AdminGetGoalTemplate(ctx context.Context, id uuid.UUID) (db.AdminGetGoalTemplateRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalTemplatesRepo.AdminGetGoalTemplate")
	defer span.End()
	return r.db.AdminGetGoalTemplate(ctx, id)
}

func (r *GoalTemplatesRepo) AdminCreateGoalTemplate(ctx context.Context, params db.AdminCreateGoalTemplateParams) (db.GoalTemplate, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalTemplatesRepo.AdminCreateGoalTemplate")
	defer span.End()
	return r.db.AdminCreateGoalTemplate(ctx, params)
}

func (r *GoalTemplatesRepo) AdminUpdateGoalTemplate(ctx context.Context, params db.AdminUpdateGoalTemplateParams) (db.GoalTemplate, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalTemplatesRepo.AdminUpdateGoalTemplate")
	defer span.End()
	return r.db.AdminUpdateGoalTemplate(ctx, params)
}

func (r *GoalTemplatesRepo) AdminDeleteGoalTemplate(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "GoalTemplatesRepo.AdminDeleteGoalTemplate")
	defer span.End()
	return r.db.AdminDeleteGoalTemplate(ctx, id)
}
