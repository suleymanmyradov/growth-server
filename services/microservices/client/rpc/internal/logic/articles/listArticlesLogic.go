package articleslogic

import (
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
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

	// Filter by category slug if provided
	if in.CategorySlug != "" {
		articles, err := l.svcCtx.Repo.Articles.ListArticlesByCategorySlug(l.ctx, in.CategorySlug, limit, offset)
		if err != nil {
			l.Errorf("Failed to list articles by category: %v", err)
return nil, status.Error(codes.Internal, "failed to list articles by category")
		}
		for _, a := range articles {
			pbArticles = append(pbArticles, convertCategorySlugRowToPbArticle(a))
		}
		totalCount, err = l.svcCtx.Repo.Articles.CountArticlesByCategorySlug(l.ctx, in.CategorySlug)
		if err != nil {
			l.Errorf("Failed to count articles by category: %v", err)
		}
	} else {
		articles, err := l.svcCtx.Repo.Articles.ListArticles(l.ctx, limit, offset)
		if err != nil {
			l.Errorf("Failed to list articles: %v", err)
return nil, status.Error(codes.Internal, "failed to list articles")
		}
		for _, a := range articles {
			pbArticles = append(pbArticles, convertListRowToPbArticle(a))
		}
		totalCount, err = l.svcCtx.Repo.Articles.CountArticles(l.ctx)
		if err != nil {
			l.Errorf("Failed to count articles: %v", err)
		}
	}

	return &client.ListArticlesResponse{
		Articles:   pbArticles,
		TotalCount: int32(totalCount),
	}, nil
}
