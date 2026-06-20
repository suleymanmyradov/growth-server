package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// WithTx returns a new ArticlesRepo backed by the given transaction.
func (r *ArticlesRepo) WithTx(tx pgx.Tx) *ArticlesRepo {
	return &ArticlesRepo{db: r.db.WithTx(tx)}
}

func (r *ArticlesRepo) ListArticles(ctx context.Context, status string, limit, offset int32) ([]db.ListArticlesRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.ListArticles")
	defer span.End()

	return r.db.ListArticles(ctx, limit, offset, status)
}

func (r *ArticlesRepo) ListArticlesWithSaved(ctx context.Context, status string, limit, offset int32, userID uuid.UUID) ([]db.ListArticlesWithSavedRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.ListArticlesWithSaved")
	defer span.End()

	return r.db.ListArticlesWithSaved(ctx, limit, offset, userID, status)
}

func (r *ArticlesRepo) ListArticlesByCategorySlug(ctx context.Context, slug string, status string, limit, offset int32) ([]db.ListArticlesByCategorySlugRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.ListArticlesByCategorySlug")
	defer span.End()

	return r.db.ListArticlesByCategorySlug(ctx, slug, limit, offset, status)
}

func (r *ArticlesRepo) ListArticlesByCategorySlugWithSaved(ctx context.Context, slug string, status string, limit, offset int32, userID uuid.UUID) ([]db.ListArticlesByCategorySlugWithSavedRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.ListArticlesByCategorySlugWithSaved")
	defer span.End()

	return r.db.ListArticlesByCategorySlugWithSaved(ctx, db.ListArticlesByCategorySlugWithSavedParams{
		Slug:   slug,
		Limit:  limit,
		Offset: offset,
		UserID: userID,
		Status: status,
	})
}

func (r *ArticlesRepo) ListArticlesByAuthor(ctx context.Context, author string, status string, limit, offset int32) ([]db.ListArticlesByAuthorRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.ListArticlesByAuthor")
	defer span.End()

	return r.db.ListArticlesByAuthor(ctx, author, limit, offset, status)
}

func (r *ArticlesRepo) ListArticlesByAuthorWithSaved(ctx context.Context, author string, status string, limit, offset int32, userID uuid.UUID) ([]db.ListArticlesByAuthorWithSavedRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.ListArticlesByAuthorWithSaved")
	defer span.End()

	return r.db.ListArticlesByAuthorWithSaved(ctx, db.ListArticlesByAuthorWithSavedParams{
		Author: author,
		Limit:  limit,
		Offset: offset,
		UserID: userID,
		Status: status,
	})
}

func (r *ArticlesRepo) SearchArticles(ctx context.Context, query string, status string, limit, offset int32) ([]db.SearchArticlesRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.SearchArticles")
	defer span.End()

	return r.db.SearchArticles(ctx, query, limit, offset, status)
}

func (r *ArticlesRepo) GetArticleByID(ctx context.Context, id uuid.UUID, status string) (db.GetArticleRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.GetArticleByID")
	defer span.End()

	return r.db.GetArticle(ctx, id, status)
}

func (r *ArticlesRepo) GetArticleByIDWithSaved(ctx context.Context, id uuid.UUID, userID uuid.UUID, status string) (db.GetArticleWithSavedRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.GetArticleByIDWithSaved")
	defer span.End()

	return r.db.GetArticleWithSaved(ctx, id, userID, status)
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

func (r *ArticlesRepo) CountArticles(ctx context.Context, status string) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.CountArticles")
	defer span.End()

	return r.db.CountArticles(ctx, status)
}

func (r *ArticlesRepo) CountArticlesByCategorySlug(ctx context.Context, slug string, status string) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.CountArticlesByCategorySlug")
	defer span.End()

	return r.db.CountArticlesByCategorySlug(ctx, slug, status)
}

func (r *ArticlesRepo) CountSearchArticles(ctx context.Context, query string, status string) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.CountSearchArticles")
	defer span.End()

	return r.db.CountSearchArticles(ctx, query, status)
}

func (r *ArticlesRepo) CreateArticleShare(ctx context.Context, articleID uuid.UUID, userID uuid.UUID, platform string) (db.ArticleShare, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.CreateArticleShare")
	defer span.End()

	return r.db.CreateArticleShare(ctx, articleID, userID, platform)
}

func (r *ArticlesRepo) CreateArticleLike(ctx context.Context, articleID uuid.UUID, userID uuid.UUID) (db.ArticleLike, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.CreateArticleLike")
	defer span.End()

	return r.db.CreateArticleLike(ctx, articleID, userID)
}

func (r *ArticlesRepo) DeleteArticleLike(ctx context.Context, articleID uuid.UUID, userID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.DeleteArticleLike")
	defer span.End()

	return r.db.DeleteArticleLike(ctx, articleID, userID)
}

func (r *ArticlesRepo) CountArticleLikes(ctx context.Context, articleID uuid.UUID) (int64, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.CountArticleLikes")
	defer span.End()

	return r.db.CountArticleLikes(ctx, articleID)
}

func (r *ArticlesRepo) IsArticleLikedByUser(ctx context.Context, articleID uuid.UUID, userID uuid.UUID) (bool, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.IsArticleLikedByUser")
	defer span.End()

	return r.db.IsArticleLikedByUser(ctx, articleID, userID)
}

func (r *ArticlesRepo) UpsertTags(ctx context.Context, names []string, slugs []string) ([]db.UpsertTagsRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.UpsertTags")
	defer span.End()

	return r.db.UpsertTags(ctx, names, slugs)
}

func (r *ArticlesRepo) DeleteArticleTags(ctx context.Context, articleID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.DeleteArticleTags")
	defer span.End()

	return r.db.DeleteArticleTags(ctx, articleID)
}

func (r *ArticlesRepo) LinkArticleTags(ctx context.Context, articleID uuid.UUID, tagNames []string) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.LinkArticleTags")
	defer span.End()

	return r.db.LinkArticleTags(ctx, articleID, tagNames)
}

func (r *ArticlesRepo) GetTagsByArticleIDs(ctx context.Context, articleIDs []uuid.UUID) ([]db.GetTagsByArticleIDsRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.GetTagsByArticleIDs")
	defer span.End()

	return r.db.GetTagsByArticleIDs(ctx, articleIDs)
}

func (r *ArticlesRepo) ListTags(ctx context.Context) ([]db.ListTagsRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "ArticlesRepo.ListTags")
	defer span.End()

	return r.db.ListTags(ctx)
}
