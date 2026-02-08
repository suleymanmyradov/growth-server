package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/pb/search"

	"github.com/zeromicro/go-zero/core/logx"
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

func (l *DeleteSearchHistoryItemLogic) DeleteSearchHistoryItem(in *search.DeleteSearchHistoryItemRequest) (*search.DeleteSearchHistoryItemResponse, error) {
	// todo: add your logic here and delete this line

	return &search.DeleteSearchHistoryItemResponse{}, nil
}
