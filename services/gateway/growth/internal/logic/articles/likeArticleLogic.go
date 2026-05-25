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

type LikeArticleLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLikeArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LikeArticleLogic {
	return &LikeArticleLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LikeArticleLogic) LikeArticle(req *types.LikeArticleRequest) (resp *types.LikeArticleResponse, err error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, nil
	}

	rpcResp, err := l.svcCtx.ArticlesRpc.LikeArticle(l.ctx, &clientarticles.LikeArticleRequest{
		ArticleId: req.Id,
		UserId:    p.UserID,
	})
	if err != nil {
		return nil, err
	}

	return &types.LikeArticleResponse{
		Success:      rpcResp.Success,
		NewLikeCount: int(rpcResp.NewLikeCount),
	}, nil
}
