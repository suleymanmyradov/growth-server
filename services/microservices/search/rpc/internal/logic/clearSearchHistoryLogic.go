package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/pb/search"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type ClearSearchHistoryLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewClearSearchHistoryLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ClearSearchHistoryLogic {
	return &ClearSearchHistoryLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ClearSearchHistoryLogic) ClearSearchHistory(_ *search.ClearSearchHistoryRequest) (*search.ClearSearchHistoryResponse, error) {
	_, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ClearSearchHistoryLogic.ClearSearchHistory")
	defer span.End()

	// todo: add your logic here and delete this line

	return &search.ClearSearchHistoryResponse{}, nil
}
