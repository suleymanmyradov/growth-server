package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/articles/rpc/articles"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/articles/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTrendingArticlesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTrendingArticlesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTrendingArticlesLogic {
	return &GetTrendingArticlesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetTrendingArticlesLogic) GetTrendingArticles(in *articles.GetTrendingArticlesRequest) (*articles.GetTrendingArticlesResponse, error) {
	// todo: add your logic here and delete this line

	return &articles.GetTrendingArticlesResponse{}, nil
}
