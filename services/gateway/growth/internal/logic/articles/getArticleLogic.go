// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package articles

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetArticleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetArticleLogic {
	return &GetArticleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetArticleLogic) GetArticle(req *types.ArticleRequest) (resp *types.ArticleResponse, err error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "article id is required")
	}

	l.Infof("Fetching article with id: %s", req.Id)

	rpcReq := &clientarticles.GetArticleRequest{
		ArticleId: req.Id,
	}

	// Pass user ID if authenticated so backend can compute isSaved
	if p, ok := principal.PrincipalFrom(l.ctx); ok {
		rpcReq.UserId = p.UserID
	}

	rpcResp, err := l.svcCtx.ArticlesRpc.GetArticle(l.ctx, rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get article from rpc: %w", err)
	}

	if rpcResp.Article == nil {
		return nil, status.Error(codes.NotFound, "article not found")
	}

	rpcArticle := rpcResp.Article

	category := l.mapCategory(rpcArticle.Category)

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
		IsSaved:     rpcArticle.IsSaved,
		LikeCount:   int(rpcArticle.Likes),
		IsLiked:     rpcArticle.IsLiked,
		Tags:        rpcArticle.Tags,
	}

	l.Infof("Successfully fetched article: %s", article.Title)

	return &types.ArticleResponse{
		Data: article,
	}, nil
}

func (l *GetArticleLogic) mapCategory(rpcCategory *client.ArticleCategory) *types.ArticleCategory {
	if rpcCategory == nil {
		return nil
	}

	return &types.ArticleCategory{
		Id:   rpcCategory.Id,
		Name: rpcCategory.Name,
		Slug: rpcCategory.Slug,
	}
}
