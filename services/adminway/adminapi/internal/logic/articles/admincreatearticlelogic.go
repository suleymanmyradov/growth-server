package articles

import (
	"context"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminCreateArticleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminCreateArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminCreateArticleLogic {
	return &AdminCreateArticleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminCreateArticleLogic) AdminCreateArticle(req *types.CreateArticleRequest) (resp *types.ArticleResponse, err error) {
	if req.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}

	rpcReq := &clientarticles.CreateArticleRequest{
		Title:      req.Title,
		Content:    req.Content,
		Summary:    req.Summary,
		AuthorId:   req.AuthorId,
		CoverImage: req.CoverImage,
		Tags:       req.Tags,
		ReadTime:   int32(req.ReadTime),
		CategoryId: req.CategoryId,
		Status:     req.Status,
	}

	rpcResp, err := l.svcCtx.ArticlesRpc.CreateArticle(l.ctx, rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create article via rpc: %w", err)
	}

	if rpcResp.Article == nil {
		return nil, status.Error(codes.Internal, "article creation returned nil")
	}

	rpcArticle := rpcResp.Article
	category := mapCategory(rpcArticle.Category)

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
		Status:      rpcArticle.Status,
	}

	return &types.ArticleResponse{
		Data: article,
	}, nil
}
