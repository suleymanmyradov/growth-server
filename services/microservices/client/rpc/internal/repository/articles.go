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

	return r.db.ListArticles(ctx, limit, offset)
}

func (r *ArticlesRepo) ListArticlesByCategorySlug(ctx context.Context, slug string, limit, offset int32) ([]db.ListArticlesByCategorySlugRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.ListArticlesByCategorySlug")
	defer span.End()

	return r.db.ListArticlesByCategorySlug(ctx, slug, limit, offset)
}

func (r *ArticlesRepo) ListArticlesByAuthor(ctx context.Context, author string, limit, offset int32) ([]db.ListArticlesByAuthorRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.ListArticlesByAuthor")
	defer span.End()

	return r.db.ListArticlesByAuthor(ctx, author, limit, offset)
}

func (r *ArticlesRepo) SearchArticles(ctx context.Context, query string, limit, offset int32) ([]db.SearchArticlesRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.SearchArticles")
	defer span.End()

	return r.db.SearchArticles(ctx, query, limit, offset)
}

func (r *ArticlesRepo) GetArticleByID(ctx context.Context, id uuid.UUID) (db.GetArticleRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.GetArticleByID")
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

func (r *ArticlesRepo) CountArticlesByCategorySlug(ctx context.Context, slug string) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.CountArticlesByCategorySlug")
	defer span.End()

	return r.db.CountArticlesByCategorySlug(ctx, slug)
}

func (r *ArticlesRepo) CreateArticleShare(ctx context.Context, articleID uuid.UUID, userID uuid.UUID, platform string) (db.ArticleShare, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.CreateArticleShare")
	defer span.End()

	return r.db.CreateArticleShare(ctx, articleID, userID, platform)
}
