package notificationslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

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

func (l *CreateNotificationLogic) CreateNotification(in *client.CreateNotificationRequest) (*client.CreateNotificationResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Logger.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	params := db.CreateNotificationParams{
		UserID:   userID,
		ItemType: in.Type,
		Title:    in.Title,
		Message:  in.Message,
	}

	notification, err := l.svcCtx.Repo.Notifications.CreateNotification(l.ctx, params)
	if err != nil {
		l.Logger.Errorf("Failed to create notification: %v", err)
		return nil, err
	}

	return &client.CreateNotificationResponse{
		NotificationId: notification.ID.String(),
	}, nil
}
