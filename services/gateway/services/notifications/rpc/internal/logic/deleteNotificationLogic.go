package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/notifications"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteNotificationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteNotificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteNotificationLogic {
	return &DeleteNotificationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteNotificationLogic) DeleteNotification(in *notifications.DeleteNotificationRequest) (*notifications.DeleteNotificationResponse, error) {
	// todo: add your logic here and delete this line

	return &notifications.DeleteNotificationResponse{}, nil
}
