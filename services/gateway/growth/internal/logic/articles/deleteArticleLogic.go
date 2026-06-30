// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package articles

import (
	"context"
	"fmt"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"
	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteArticleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteArticleLogic {
	return &DeleteArticleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteArticleLogic) DeleteArticle(req *types.ArticleRequest) (resp *types.EmptyResponse, err error) {
	rpcReq := &clientarticles.DeleteArticleRequest{
		ArticleId: req.Id,
	}

	_, err = l.svcCtx.ArticlesRpc.DeleteArticle(l.ctx, rpcReq)
	if err != nil {
		return nil, fmt.Errorf("failed to delete article via rpc: %w", err)
	}

	return &types.EmptyResponse{}, nil
}
