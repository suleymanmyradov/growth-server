package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/pb/search"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
func (l *GetSearchHistoryLogic) GetSearchHistory(_ *search.GetSearchHistoryRequest) (*search.GetSearchHistoryResponse, error) {
	_, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetSearchHistoryLogic.GetSearchHistory")
	defer span.End()

	// todo: add your logic here and delete this line

	return &search.GetSearchHistoryResponse{}, nil
}
