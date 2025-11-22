package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/articles/rpc/articles"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/articles/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateArticleLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateArticleLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateArticleLogic {
	return &CreateArticleLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Author operations
func (l *CreateArticleLogic) CreateArticle(in *articles.CreateArticleRequest) (*articles.CreateArticleResponse, error) {
	// todo: add your logic here and delete this line

	return &articles.CreateArticleResponse{}, nil
}
