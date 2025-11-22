package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/articles/rpc/articles"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/articles/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ShareArticleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewShareArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ShareArticleLogic {
	return &ShareArticleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ShareArticleLogic) ShareArticle(in *articles.ShareArticleRequest) (*articles.ShareArticleResponse, error) {
	// todo: add your logic here and delete this line

	return &articles.ShareArticleResponse{}, nil
}
