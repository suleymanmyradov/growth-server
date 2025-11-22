package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/notifications"

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

// Preferences
func (l *GetNotificationPreferencesLogic) GetNotificationPreferences(in *notifications.GetNotificationPreferencesRequest) (*notifications.GetNotificationPreferencesResponse, error) {
	// todo: add your logic here and delete this line

	return &notifications.GetNotificationPreferencesResponse{}, nil
}
