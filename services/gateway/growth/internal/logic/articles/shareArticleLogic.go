// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package articles

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"

	"github.com/zeromicro/go-zero/core/logx"
)

type ShareArticleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewShareArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ShareArticleLogic {
	return &ShareArticleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ShareArticleLogic) ShareArticle(req *types.ShareArticleRequest) (resp *types.ShareArticleResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, nil
	}

	rpcResp, err := l.svcCtx.ArticlesRpc.ShareArticle(l.ctx, &clientarticles.ShareArticleRequest{
		ArticleId: req.Id,
		UserId:    p.UserID,
		Platform:  req.Platform,
	})
	if err != nil {
		return nil, err
	}

	return &types.ShareArticleResponse{
		Success: rpcResp.Success,
	}, nil
}
