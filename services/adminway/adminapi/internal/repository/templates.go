package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/repository/db"
)

type IHabitTemplates interface {
	List(ctx context.Context) ([]db.AdminListHabitTemplatesRow, error)
	Get(ctx context.Context, id uuid.UUID) (db.AdminGetHabitTemplateRow, error)
	Create(ctx context.Context, params db.AdminCreateHabitTemplateParams) (db.HabitTemplate, error)
	Update(ctx context.Context, params db.AdminUpdateHabitTemplateParams) (db.HabitTemplate, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type IGoalTemplates interface {
	List(ctx context.Context) ([]db.AdminListGoalTemplatesRow, error)
	Get(ctx context.Context, id uuid.UUID) (db.AdminGetGoalTemplateRow, error)
	Create(ctx context.Context, params db.AdminCreateGoalTemplateParams) (db.GoalTemplate, error)
	Update(ctx context.Context, params db.AdminUpdateGoalTemplateParams) (db.GoalTemplate, error)
	Delete(ctx context.Context, id uuid.UUID) error
}

type HabitTemplatesRepo struct {
	db *db.Queries
}

func NewHabitTemplatesRepo(dbq *db.Queries) *HabitTemplatesRepo {
	return &HabitTemplatesRepo{db: dbq}
}

func (r *HabitTemplatesRepo) List(ctx context.Context) ([]db.AdminListHabitTemplatesRow, error) {
	return r.db.AdminListHabitTemplates(ctx)
}

func (r *HabitTemplatesRepo) Get(ctx context.Context, id uuid.UUID) (db.AdminGetHabitTemplateRow, error) {
	return r.db.AdminGetHabitTemplate(ctx, id)
}

func (r *HabitTemplatesRepo) Create(ctx context.Context, params db.AdminCreateHabitTemplateParams) (db.HabitTemplate, error) {
	return r.db.AdminCreateHabitTemplate(ctx, params)
}

func (r *HabitTemplatesRepo) Update(ctx context.Context, params db.AdminUpdateHabitTemplateParams) (db.HabitTemplate, error) {
	return r.db.AdminUpdateHabitTemplate(ctx, params)
}

func (r *HabitTemplatesRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.AdminDeleteHabitTemplate(ctx, id)
}

type GoalTemplatesRepo struct {
	db *db.Queries
}

func NewGoalTemplatesRepo(dbq *db.Queries) *GoalTemplatesRepo {
	return &GoalTemplatesRepo{db: dbq}
}

func (r *GoalTemplatesRepo) List(ctx context.Context) ([]db.AdminListGoalTemplatesRow, error) {
	return r.db.AdminListGoalTemplates(ctx)
}

func (r *GoalTemplatesRepo) Get(ctx context.Context, id uuid.UUID) (db.AdminGetGoalTemplateRow, error) {
	return r.db.AdminGetGoalTemplate(ctx, id)
}

func (r *GoalTemplatesRepo) Create(ctx context.Context, params db.AdminCreateGoalTemplateParams) (db.GoalTemplate, error) {
	return r.db.AdminCreateGoalTemplate(ctx, params)
}

func (r *GoalTemplatesRepo) Update(ctx context.Context, params db.AdminUpdateGoalTemplateParams) (db.GoalTemplate, error) {
	return r.db.AdminUpdateGoalTemplate(ctx, params)
}

func (r *GoalTemplatesRepo) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.AdminDeleteGoalTemplate(ctx, id)
}
