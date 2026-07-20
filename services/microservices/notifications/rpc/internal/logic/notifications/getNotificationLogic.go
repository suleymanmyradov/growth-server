package notificationslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNotificationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetNotificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNotificationLogic {
	return &GetNotificationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetNotificationLogic) GetNotification(in *notifications.GetNotificationRequest) (*notifications.GetNotificationResponse, error) {
	// todo: add your logic here and delete this line

	return &notifications.GetNotificationResponse{}, nil
}
