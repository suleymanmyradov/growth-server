package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/saved/rpc/saved"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSavedItemLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSavedItemLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSavedItemLogic {
	return &GetSavedItemLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSavedItemLogic) GetSavedItem(in *saved.GetSavedItemRequest) (*saved.GetSavedItemResponse, error) {
	// todo: add your logic here and delete this line

	return &saved.GetSavedItemResponse{}, nil
}
