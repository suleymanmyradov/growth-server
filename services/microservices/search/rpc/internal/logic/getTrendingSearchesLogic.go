package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/pb/search"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
func (l *GetTrendingSearchesLogic) GetTrendingSearches(_ *search.GetTrendingSearchesRequest) (*search.GetTrendingSearchesResponse, error) {
	_, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetTrendingSearchesLogic.GetTrendingSearches")
	defer span.End()

	// todo: add your logic here and delete this line

	return &search.GetTrendingSearchesResponse{}, nil
}
