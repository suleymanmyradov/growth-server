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

type AdminGetArticleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminGetArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminGetArticleLogic {
	return &AdminGetArticleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminGetArticleLogic) AdminGetArticle(req *types.ArticleRequest) (resp *types.ArticleResponse, err error) {
	if req.Id == "" {
		return nil, status.Error(codes.InvalidArgument, "article id is required")
	}

	rpcReq := &clientarticles.GetArticleRequest{
		ArticleId: req.Id,
	}

	rpcResp, err := l.svcCtx.ArticlesRpc.GetArticle(l.ctx, rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to get article from rpc: %w", err)
	}

	if rpcResp.Article == nil {
		return nil, status.Error(codes.NotFound, "article not found")
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
		LikeCount:   int(rpcArticle.Likes),
		IsLiked:     rpcArticle.IsLiked,
	}

	return &types.ArticleResponse{
		Data: article,
	}, nil
}
