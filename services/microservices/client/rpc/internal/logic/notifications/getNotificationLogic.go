package notificationslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

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

func (l *GetNotificationLogic) GetNotification(in *client.GetNotificationRequest) (*client.GetNotificationResponse, error) {
	notificationID, err := uuid.Parse(in.NotificationId)
	if err != nil {
		l.Logger.Errorf("Invalid notification ID: %v", err)
		return nil, err
	}

	notification, err := l.svcCtx.Repo.Notifications.GetNotification(l.ctx, notificationID)
	if err != nil {
		l.Logger.Errorf("Failed to get notification: %v", err)
		return nil, err
	}

	pb := &client.Notification{
		Id:        notification.ID.String(),
		UserId:    notification.UserID.String(),
		Type:      notification.ItemType,
		Title:     notification.Title,
		Message:   notification.Message,
		CreatedAt: notification.CreatedAt.Time.Unix(),
	}
	if notification.Read.Valid {
		pb.Read = notification.Read.Bool
	}

	return &client.GetNotificationResponse{
		Notification: pb,
	}, nil
}
