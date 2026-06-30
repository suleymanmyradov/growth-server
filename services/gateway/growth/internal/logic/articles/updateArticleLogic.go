// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package articles

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateArticleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateArticleLogic {
	return &UpdateArticleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateArticleLogic) UpdateArticle(req *types.UpdateArticleRequest) (resp *types.ArticleResponse, err error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "article id is required")
	}

	rpcReq := &clientarticles.UpdateArticleRequest{
		ArticleId:  req.Id,
		Title:      req.Title,
		Content:    req.Content,
		Summary:    req.Summary,
		AuthorId:   req.AuthorId,
		CoverImage: req.CoverImage,
		Tags:       req.Tags,
		ReadTime:   int32(req.ReadTime),
		CategoryId: req.CategoryId,
	}

	rpcResp, err := l.svcCtx.ArticlesRpc.UpdateArticle(l.ctx, rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to update article via rpc: %w", err)
	}

	if rpcResp.Article == nil {
		return nil, status.Error(codes.Internal, "article update returned nil")
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
		Tags:        rpcArticle.Tags,
	}

	return &types.ArticleResponse{
		Data: article,
	}, nil
}

func (l *UpdateArticleLogic) mapCategory(rpcCategory *client.ArticleCategory) *types.ArticleCategory {
	if rpcCategory == nil {
		return nil
	}
	return &types.ArticleCategory{
		Id:   rpcCategory.Id,
		Name: rpcCategory.Name,
		Slug: rpcCategory.Slug,
	}
}
