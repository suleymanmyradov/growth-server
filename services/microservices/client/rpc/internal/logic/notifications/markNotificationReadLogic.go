package notificationslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type MarkNotificationReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMarkNotificationReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkNotificationReadLogic {
	return &MarkNotificationReadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *MarkNotificationReadLogic) MarkNotificationRead(in *client.MarkNotificationReadRequest) (*client.MarkNotificationReadResponse, error) {
	notificationID, err := uuid.Parse(in.NotificationId)
	if err != nil {
		l.Logger.Errorf("Invalid notification ID: %v", err)
		return nil, err
	}

	_, err = l.svcCtx.Repo.Notifications.MarkNotificationRead(l.ctx, notificationID)
	if err != nil {
		l.Logger.Errorf("Failed to mark notification as read: %v", err)
		return nil, err
	}

	return &client.MarkNotificationReadResponse{
		Success: true,
	}, nil
}
