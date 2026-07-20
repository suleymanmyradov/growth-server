// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package articles

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetFeaturedArticleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetFeaturedArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetFeaturedArticleLogic {
	return &GetFeaturedArticleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetFeaturedArticleLogic) GetFeaturedArticle() (resp *types.ArticleResponse, err error) {
	rpcResp, err := l.svcCtx.ClientRpc.Articles.GetFeaturedArticle(l.ctx, &clientarticles.GetFeaturedArticleRequest{})
	if err != nil {
		return nil, err
	}

	// No featured article available (e.g. empty table). Return nil data so the
	// frontend can render its fallback.
	if rpcResp.Article == nil {
		return &types.ArticleResponse{}, nil
	}

	rpcArticle := rpcResp.Article
	category := mapArticleCategory(rpcArticle.Category)

	article := types.Article{
		Id:          rpcArticle.Id,
		Title:       rpcArticle.Title,
		Excerpt:     rpcArticle.Summary,
		Content:     rpcArticle.Content,
		Category:    category,
		ReadTime:    int(rpcArticle.ReadTime),
		ImageUrl:    rpcArticle.CoverImage,
		Author:      rpcArticle.AuthorId,
		PublishedAt: formatTime(rpcArticle.PublishedAt),
		CreatedAt:   formatTime(rpcArticle.CreatedAt),
		UpdatedAt:   formatTime(rpcArticle.UpdatedAt),
		LikeCount:   int(rpcArticle.Likes),
		Tags:        nonNilTags(rpcArticle.Tags),
	}

	return &types.ArticleResponse{
		Data: article,
	}, nil
}

func mapArticleCategory(rpcCategory *client.ArticleCategory) *types.ArticleCategory {
	if rpcCategory == nil {
		return nil
	}
	return &types.ArticleCategory{
		Id:   rpcCategory.Id,
		Name: rpcCategory.Name,
		Slug: rpcCategory.Slug,
	}
}
