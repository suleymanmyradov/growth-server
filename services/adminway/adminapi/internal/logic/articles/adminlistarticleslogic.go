package articles

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminListArticlesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminListArticlesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminListArticlesLogic {
	return &AdminListArticlesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminListArticlesLogic) AdminListArticles(req *types.ListArticlesRequest) (resp *types.ArticlesResponse, err error) {
	offset := (req.Page - 1) * req.Limit

	// Server-side full-text search takes precedence when a search term is provided.
	if req.Search != "" {
		rpcResp, err := l.svcCtx.ArticlesRpc.SearchArticles(l.ctx, &clientarticles.SearchArticlesRequest{
			Query:  req.Search,
			Status: req.Status,
			Limit:  int32(req.Limit),
			Offset: int32(offset),
		})
		if err != nil {
			return nil, err
		}

		articles := mapRpcArticles(rpcResp.Articles)
		totalPages := totalPages(int(rpcResp.TotalCount), req.Limit)

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

	rpcReq := &clientarticles.ListArticlesRequest{
		CategorySlug: req.CategorySlug,
		Status:       req.Status,
		Offset:       int32(offset),
		Limit:        int32(req.Limit),
	}

	rpcResp, err := l.svcCtx.ArticlesRpc.ListArticles(l.ctx, rpcReq)
	if err != nil {
		return nil, err
	}

	articles := mapRpcArticles(rpcResp.Articles)
	totalPages := totalPages(int(rpcResp.TotalCount), req.Limit)

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

func totalPages(total, limit int) int {
	if limit <= 0 {
		return 1
	}
	p := total / limit
	if total%limit > 0 {
		p++
	}
	if p == 0 {
		p = 1
	}
	return p
}

func mapRpcArticles(rpcArticles []*client.Article) []types.Article {
	articles := make([]types.Article, 0, len(rpcArticles))
	for _, a := range rpcArticles {
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
			IsSaved:     a.IsSaved,
			LikeCount:   int(a.Likes),
			IsLiked:     a.IsLiked,
			Tags:        a.Tags,
			Status:      a.Status,
		})
	}
	return articles
}
