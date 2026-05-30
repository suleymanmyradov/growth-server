package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// CategoriesRepo implements ICategories interface
type CategoriesRepo struct {
	db *db.Queries
}

// NewCategoriesRepo creates a new CategoriesRepo instance
func NewCategoriesRepo(db *db.Queries) *CategoriesRepo {
	return &CategoriesRepo{db: db}
}

func (r *CategoriesRepo) ListCategories(ctx context.Context, entityType db.EntityType) ([]db.Category, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CategoriesRepo.ListCategories")
	defer span.End()

	return r.db.ListCategories(ctx, entityType)
}

func (r *CategoriesRepo) ListAllCategories(ctx context.Context) ([]db.Category, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CategoriesRepo.ListAllCategories")
	defer span.End()

	return r.db.ListAllCategories(ctx)
}

func (r *CategoriesRepo) GetCategoryByID(ctx context.Context, id uuid.UUID) (db.Category, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CategoriesRepo.GetCategoryByID")
	defer span.End()

	return r.db.GetCategory(ctx, id)
}

func (r *CategoriesRepo) GetCategoryBySlug(ctx context.Context, slug string, entityType db.EntityType) (db.Category, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CategoriesRepo.GetCategoryBySlug")
	defer span.End()

	return r.db.GetCategoryBySlug(ctx, slug, entityType)
}

func (r *CategoriesRepo) CreateCategory(ctx context.Context, name string, slug string, entityType db.EntityType, sortOrder int32) (db.Category, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CategoriesRepo.CreateCategory")
	defer span.End()

	return r.db.CreateCategory(ctx, name, slug, entityType, sortOrder)
}

func (r *CategoriesRepo) UpdateCategory(ctx context.Context, id uuid.UUID, name string, slug string, sortOrder int32) (db.Category, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CategoriesRepo.UpdateCategory")
	defer span.End()

	return r.db.UpdateCategory(ctx, id, name, slug, sortOrder)
}

func (r *CategoriesRepo) DeleteCategory(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CategoriesRepo.DeleteCategory")
	defer span.End()

	return r.db.DeleteCategory(ctx, id)
}

func (r *CategoriesRepo) CountCategoriesByType(ctx context.Context, entityType db.EntityType) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CategoriesRepo.CountCategoriesByType")
	defer span.End()

	return r.db.CountCategoriesByType(ctx, entityType)
}
