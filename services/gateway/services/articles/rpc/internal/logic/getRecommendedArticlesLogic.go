package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/articles/rpc/articles"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/articles/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetRecommendedArticlesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetRecommendedArticlesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetRecommendedArticlesLogic {
	return &GetRecommendedArticlesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetRecommendedArticlesLogic) GetRecommendedArticles(in *articles.GetRecommendedArticlesRequest) (*articles.GetRecommendedArticlesResponse, error) {
	// todo: add your logic here and delete this line

	return &articles.GetRecommendedArticlesResponse{}, nil
}
