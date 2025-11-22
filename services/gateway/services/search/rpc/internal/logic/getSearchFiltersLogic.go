package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/search/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/search/rpc/search"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSearchFiltersLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSearchFiltersLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSearchFiltersLogic {
	return &GetSearchFiltersLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSearchFiltersLogic) GetSearchFilters(in *search.GetSearchFiltersRequest) (*search.GetSearchFiltersResponse, error) {
	// todo: add your logic here and delete this line

	return &search.GetSearchFiltersResponse{}, nil
}
