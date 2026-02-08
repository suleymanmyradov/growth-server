package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/trace"
)

// ArticlesRepo implements IArticles interface
type ArticlesRepo struct {
	db *db.Queries
}

// NewArticlesRepo creates a new ArticlesRepo instance
func NewArticlesRepo(db *db.Queries) *ArticlesRepo {
	return &ArticlesRepo{db: db}
}

func (r *ArticlesRepo) ListArticles(ctx context.Context, limit, offset int32) ([]db.ListArticlesRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.ListArticles")
	defer span.End()

	return r.db.ListArticles(ctx, db.ListArticlesParams{
		Limit:  limit,
		Offset: offset,
	})
}

func (r *ArticlesRepo) ListArticlesByCategory(ctx context.Context, category string, limit, offset int32) ([]db.ListArticlesByCategoryRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.ListArticlesByCategory")
	defer span.End()

	return r.db.ListArticlesByCategory(ctx, db.ListArticlesByCategoryParams{
		Category: category,
		Limit:    limit,
		Offset:   offset,
	})
}

func (r *ArticlesRepo) ListArticlesByAuthor(ctx context.Context, author string, limit, offset int32) ([]db.ListArticlesByAuthorRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.ListArticlesByAuthor")
	defer span.End()

	return r.db.ListArticlesByAuthor(ctx, db.ListArticlesByAuthorParams{
		Author: author,
		Limit:  limit,
		Offset: offset,
	})
}

func (r *ArticlesRepo) SearchArticles(ctx context.Context, query string, limit, offset int32) ([]db.SearchArticlesRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.SearchArticles")
	defer span.End()

	return r.db.SearchArticles(ctx, db.SearchArticlesParams{
		PlaintoTsquery: query,
		Limit:          limit,
		Offset:         offset,
	})
}

func (r *ArticlesRepo) GetArticle(ctx context.Context, id uuid.UUID) (db.GetArticleRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.GetArticle")
	defer span.End()

	return r.db.GetArticle(ctx, id)
}

func (r *ArticlesRepo) GetArticleByTitle(ctx context.Context, title string) (db.GetArticleByTitleRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.GetArticleByTitle")
	defer span.End()

	return r.db.GetArticleByTitle(ctx, title)
}

func (r *ArticlesRepo) CreateArticle(ctx context.Context, params db.CreateArticleParams) (db.CreateArticleRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.CreateArticle")
	defer span.End()

	return r.db.CreateArticle(ctx, params)
}

func (r *ArticlesRepo) UpdateArticle(ctx context.Context, params db.UpdateArticleParams) (db.UpdateArticleRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.UpdateArticle")
	defer span.End()

	return r.db.UpdateArticle(ctx, params)
}

func (r *ArticlesRepo) DeleteArticle(ctx context.Context, id uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.DeleteArticle")
	defer span.End()

	return r.db.DeleteArticle(ctx, id)
}

func (r *ArticlesRepo) CountArticles(ctx context.Context) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.CountArticles")
	defer span.End()

	return r.db.CountArticles(ctx)
}

func (r *ArticlesRepo) CountArticlesByCategory(ctx context.Context, category string) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.CountArticlesByCategory")
	defer span.End()

	return r.db.CountArticlesByCategory(ctx, category)
}

func (r *ArticlesRepo) CreateArticleShare(ctx context.Context, params db.CreateArticleShareParams) (db.ArticleShare, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.CreateArticleShare")
	defer span.End()

	return r.db.CreateArticleShare(ctx, params)
}
