package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/search/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/search/rpc/search"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetTrendingSearchesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetTrendingSearchesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetTrendingSearchesLogic {
	return &GetTrendingSearchesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Analytics
func (l *GetTrendingSearchesLogic) GetTrendingSearches(in *search.GetTrendingSearchesRequest) (*search.GetTrendingSearchesResponse, error) {
	// todo: add your logic here and delete this line

	return &search.GetTrendingSearchesResponse{}, nil
}
