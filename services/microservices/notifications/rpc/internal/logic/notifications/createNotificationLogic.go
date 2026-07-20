package notificationslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateNotificationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateNotificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateNotificationLogic {
	return &CreateNotificationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateNotificationLogic) CreateNotification(in *notifications.CreateNotificationRequest) (*notifications.CreateNotificationResponse, error) {
	// todo: add your logic here and delete this line

	return &notifications.CreateNotificationResponse{}, nil
}
