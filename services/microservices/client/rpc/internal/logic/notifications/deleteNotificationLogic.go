package notificationslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

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

func (l *DeleteNotificationLogic) DeleteNotification(in *client.DeleteNotificationRequest) (*client.DeleteNotificationResponse, error) {
	notificationID, err := uuid.Parse(in.NotificationId)
	if err != nil {
		l.Logger.Errorf("Invalid notification ID: %v", err)
		return nil, err
	}

	err = l.svcCtx.Repo.Notifications.DeleteNotification(l.ctx, notificationID)
	if err != nil {
		l.Logger.Errorf("Failed to delete notification: %v", err)
		return nil, err
	}

	return &client.DeleteNotificationResponse{
		Success: true,
	}, nil
}
