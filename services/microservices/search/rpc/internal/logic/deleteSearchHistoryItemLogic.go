package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/pb/search"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type DeleteSearchHistoryItemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteSearchHistoryItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteSearchHistoryItemLogic {
	return &DeleteSearchHistoryItemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteSearchHistoryItemLogic) DeleteSearchHistoryItem(_ *search.DeleteSearchHistoryItemRequest) (*search.DeleteSearchHistoryItemResponse, error) {
	_, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "DeleteSearchHistoryItemLogic.DeleteSearchHistoryItem")
	defer span.End()

	// todo: add your logic here and delete this line

	return &search.DeleteSearchHistoryItemResponse{}, nil
}
