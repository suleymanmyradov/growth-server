package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/notifications"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNotificationTypesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetNotificationTypesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNotificationTypesLogic {
	return &GetNotificationTypesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetNotificationTypesLogic) GetNotificationTypes(in *notifications.GetNotificationTypesRequest) (*notifications.GetNotificationTypesResponse, error) {
	// todo: add your logic here and delete this line

	return &notifications.GetNotificationTypesResponse{}, nil
}
