package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// TagsRepo implements ITags interface
type TagsRepo struct {
	db *db.Queries
}

// NewTagsRepo creates a new TagsRepo instance
func NewTagsRepo(db *db.Queries) *TagsRepo {
	return &TagsRepo{db: db}
}

// WithTx returns a new TagsRepo backed by the given transaction.
func (r *TagsRepo) WithTx(tx pgx.Tx) *TagsRepo {
	return &TagsRepo{db: r.db.WithTx(tx)}
}

func (r *TagsRepo) CreateTag(ctx context.Context, name string, slug string) (db.Tag, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "TagsRepo.CreateTag")
	defer span.End()

	return r.db.CreateTag(ctx, name, slug)
}

func (r *TagsRepo) GetTag(ctx context.Context, id uuid.UUID) (db.Tag, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "TagsRepo.GetTag")
	defer span.End()

	return r.db.GetTag(ctx, id)
}

func (r *TagsRepo) GetTagBySlug(ctx context.Context, slug string) (db.Tag, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "TagsRepo.GetTagBySlug")
	defer span.End()

	return r.db.GetTagBySlug(ctx, slug)
}

func (r *TagsRepo) UpdateTag(ctx context.Context, id uuid.UUID, name string, slug string) (db.Tag, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "TagsRepo.UpdateTag")
	defer span.End()

	return r.db.UpdateTag(ctx, id, name, slug)
}

func (r *TagsRepo) DeleteTag(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "TagsRepo.DeleteTag")
	defer span.End()

	return r.db.DeleteTag(ctx, id)
}

func (r *TagsRepo) CountTagUsage(ctx context.Context, id uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "TagsRepo.CountTagUsage")
	defer span.End()

	return r.db.CountTagUsage(ctx, id)
}
