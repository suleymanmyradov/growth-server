package articles

import (
	"context"
	"fmt"

	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/adminway/adminapi/internal/types"
	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"

	"github.com/zeromicro/go-zero/core/logx"
)

type AdminDeleteArticleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewAdminDeleteArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AdminDeleteArticleLogic {
	return &AdminDeleteArticleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *AdminDeleteArticleLogic) AdminDeleteArticle(req *types.ArticleRequest) (resp *types.EmptyResponse, err error) {
	_, err = l.svcCtx.ArticlesRpc.DeleteArticle(l.ctx, &clientarticles.DeleteArticleRequest{
		ArticleId: req.Id,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to delete article via rpc: %w", err)
	}

	return &types.EmptyResponse{}, nil
}
