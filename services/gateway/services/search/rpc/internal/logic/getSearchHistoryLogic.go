package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/search/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/search/rpc/search"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSearchHistoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSearchHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSearchHistoryLogic {
	return &GetSearchHistoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Search history
func (l *GetSearchHistoryLogic) GetSearchHistory(in *search.GetSearchHistoryRequest) (*search.GetSearchHistoryResponse, error) {
	// todo: add your logic here and delete this line

	return &search.GetSearchHistoryResponse{}, nil
}
