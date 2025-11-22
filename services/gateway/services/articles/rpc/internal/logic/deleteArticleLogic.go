package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/articles/rpc/articles"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/articles/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteArticleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteArticleLogic {
	return &DeleteArticleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteArticleLogic) DeleteArticle(in *articles.DeleteArticleRequest) (*articles.DeleteArticleResponse, error) {
	// todo: add your logic here and delete this line

	return &articles.DeleteArticleResponse{}, nil
}
