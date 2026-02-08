package notificationslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateNotificationPreferencesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateNotificationPreferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateNotificationPreferencesLogic {
	return &UpdateNotificationPreferencesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateNotificationPreferencesLogic) UpdateNotificationPreferences(in *client.UpdateNotificationPreferencesRequest) (*client.UpdateNotificationPreferencesResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Logger.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	l.Logger.Infof("Updating notification preferences for user %s", userID)

	return &client.UpdateNotificationPreferencesResponse{
		Success: true,
	}, nil
}
