package articles

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"

	"github.com/zeromicro/go-zero/core/logx"
)

type ListArticlesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewListArticlesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListArticlesLogic {
	return &ListArticlesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ListArticlesLogic) ListArticles(req *types.ListArticlesRequest) (resp *types.ArticlesResponse, err error) {
	offset := (req.Page - 1) * req.Limit
	rpcResp, err := l.svcCtx.ArticlesRpc.ListArticles(l.ctx, &clientarticles.ListArticlesRequest{
		CategorySlug: req.CategorySlug,
		Offset:       int32(offset),
		Limit:        int32(req.Limit),
	})
	if err != nil {
		return nil, err
	}

	var articles []types.Article
	for _, a := range rpcResp.Articles {
		var category *types.ArticleCategory
		if a.Category != nil {
			category = &types.ArticleCategory{
				Id:   a.Category.Id,
				Name: a.Category.Name,
				Slug: a.Category.Slug,
			}
		}
		articles = append(articles, types.Article{
			Id:          a.Id,
			Title:       a.Title,
			Excerpt:     a.Summary,
			Content:     a.Content,
			Category:    category,
			ReadTime:    int(a.ReadTime),
			ImageUrl:    a.CoverImage,
			Author:      a.AuthorId,
			PublishedAt: formatTime(a.PublishedAt),
			CreatedAt:   formatTime(a.CreatedAt),
			UpdatedAt:   formatTime(a.UpdatedAt),
		})
	}

	totalPages := int(rpcResp.TotalCount) / req.Limit
	if int(rpcResp.TotalCount)%req.Limit > 0 {
		totalPages++
	}

	return &types.ArticlesResponse{
		Data: articles,
		Page: types.PageResponse{
			Total:      int64(rpcResp.TotalCount),
			Page:       req.Page,
			Limit:      req.Limit,
			TotalPages: totalPages,
		},
	}, nil
}
