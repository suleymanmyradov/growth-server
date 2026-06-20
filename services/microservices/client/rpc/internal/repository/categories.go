package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// WithTx returns a new CategoriesRepo backed by the given transaction.
func (r *CategoriesRepo) WithTx(tx pgx.Tx) *CategoriesRepo {
	return &CategoriesRepo{db: r.db.WithTx(tx)}
}

func (r *CategoriesRepo) ListCategories(ctx context.Context) ([]db.Category, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CategoriesRepo.ListCategories")
	defer span.End()

	return r.db.ListCategories(ctx)
}

func (r *CategoriesRepo) GetCategoryByID(ctx context.Context, id uuid.UUID) (db.Category, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CategoriesRepo.GetCategoryByID")
	defer span.End()

	return r.db.GetCategory(ctx, id)
}

func (r *CategoriesRepo) GetCategoryBySlug(ctx context.Context, slug string) (db.Category, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CategoriesRepo.GetCategoryBySlug")
	defer span.End()

	return r.db.GetCategoryBySlug(ctx, slug)
}

func (r *CategoriesRepo) CreateCategory(ctx context.Context, name string, slug string, sortOrder int32) (db.Category, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CategoriesRepo.CreateCategory")
	defer span.End()

	return r.db.CreateCategory(ctx, name, slug, sortOrder)
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

func (r *CategoriesRepo) CountCategories(ctx context.Context) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CategoriesRepo.CountCategories")
	defer span.End()

	return r.db.CountCategories(ctx)
}

func (r *CategoriesRepo) CountArticlesByCategory(ctx context.Context, id uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CategoriesRepo.CountArticlesByCategory")
	defer span.End()

	return r.db.CountArticlesByCategory(ctx, uuid.NullUUID{UUID: id, Valid: true})
}

func (r *CategoriesRepo) ReorderCategories(ctx context.Context, ids []uuid.UUID, sortOrders []int32) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "CategoriesRepo.ReorderCategories")
	defer span.End()

	return r.db.ReorderCategories(ctx, ids, sortOrders)
}
