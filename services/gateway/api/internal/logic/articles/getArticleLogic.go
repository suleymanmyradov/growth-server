// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package articles

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/types"

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
	// todo: add your logic here and delete this line

	return
}
