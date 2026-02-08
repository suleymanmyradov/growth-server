package notificationslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNotificationPreferencesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetNotificationPreferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNotificationPreferencesLogic {
	return &GetNotificationPreferencesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetNotificationPreferencesLogic) GetNotificationPreferences(in *client.GetNotificationPreferencesRequest) (*client.GetNotificationPreferencesResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Logger.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	l.Logger.Infof("Getting notification preferences for user %s", userID)

	return &client.GetNotificationPreferencesResponse{}, nil
}
