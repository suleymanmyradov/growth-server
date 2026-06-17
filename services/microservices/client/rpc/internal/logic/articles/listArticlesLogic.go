package articleslogic

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type ListArticlesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListArticlesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListArticlesLogic {
	return &ListArticlesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListArticlesLogic) ListArticles(in *client.ListArticlesRequest) (*client.ListArticlesResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ListArticlesLogic.ListArticles")
	defer span.End()
	limit := int32(20)
	offset := int32(0)
	if in.Limit > 0 {
		limit = in.Limit
	}
	if in.Offset > 0 {
		offset = in.Offset
	}

	var pbArticles []*client.Article
	var totalCount int64

	// Parse user ID if provided
	userID, userErr := uuid.Parse(in.UserId)
	hasUser := in.UserId != "" && userErr == nil

	// Filter by category slug if provided
	if in.CategorySlug != "" {
		if hasUser {
			articles, err := l.svcCtx.Repo.Articles.ListArticlesByCategorySlugWithSaved(ctx, in.CategorySlug, limit, offset, userID)
			if err != nil {
				l.Errorf("Failed to list articles by category with saved: %v", err)
				return nil, status.Error(codes.Internal, "failed to list articles by category")
			}
			pbArticles = make([]*client.Article, len(articles))
			for i, a := range articles {
				pbArticles[i] = convertCategorySlugWithSavedRowToPbArticle(a)
			}
		} else {
			articles, err := l.svcCtx.Repo.Articles.ListArticlesByCategorySlug(ctx, in.CategorySlug, limit, offset)
			if err != nil {
				l.Errorf("Failed to list articles by category: %v", err)
				return nil, status.Error(codes.Internal, "failed to list articles by category")
			}
			pbArticles = make([]*client.Article, len(articles))
			for i, a := range articles {
				pbArticles[i] = convertCategorySlugRowToPbArticle(a)
			}
		}
		totalCount, _ = l.svcCtx.Repo.Articles.CountArticlesByCategorySlug(ctx, in.CategorySlug)
	} else {
		if hasUser {
			articles, err := l.svcCtx.Repo.Articles.ListArticlesWithSaved(ctx, limit, offset, userID)
			if err != nil {
				l.Errorf("Failed to list articles with saved: %v", err)
				return nil, status.Error(codes.Internal, "failed to list articles")
			}
			pbArticles = make([]*client.Article, len(articles))
			for i, a := range articles {
				pbArticles[i] = convertListWithSavedRowToPbArticle(a)
			}
		} else {
			articles, err := l.svcCtx.Repo.Articles.ListArticles(ctx, limit, offset)
			if err != nil {
				l.Errorf("Failed to list articles: %v", err)
				return nil, status.Error(codes.Internal, "failed to list articles")
			}
			pbArticles = make([]*client.Article, len(articles))
			for i, a := range articles {
				pbArticles[i] = convertListRowToPbArticle(a)
			}
		}
		totalCount, _ = l.svcCtx.Repo.Articles.CountArticles(ctx)
	}

	if len(pbArticles) > 0 {
		articleIDs := make([]uuid.UUID, 0, len(pbArticles))
		for _, a := range pbArticles {
			if id, err := uuid.Parse(a.Id); err == nil {
				articleIDs = append(articleIDs, id)
			}
		}
		tagRows, err := l.svcCtx.Repo.Articles.GetTagsByArticleIDs(ctx, articleIDs)
		if err != nil {
			l.Errorf("failed to get article tags: %v", err)
		} else {
			tagMap := make(map[string][]string)
			for _, t := range tagRows {
				tagMap[t.ArticleID.String()] = append(tagMap[t.ArticleID.String()], t.Name)
			}
			for _, a := range pbArticles {
				a.Tags = tagMap[a.Id]
			}
		}
	}

	return &client.ListArticlesResponse{
		Articles:   pbArticles,
		TotalCount: int32(totalCount),
	}, nil
}
