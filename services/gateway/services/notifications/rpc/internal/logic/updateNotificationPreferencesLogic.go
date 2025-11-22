package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/notifications"

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

func (l *UpdateNotificationPreferencesLogic) UpdateNotificationPreferences(in *notifications.UpdateNotificationPreferencesRequest) (*notifications.UpdateNotificationPreferencesResponse, error) {
	// todo: add your logic here and delete this line

	return &notifications.UpdateNotificationPreferencesResponse{}, nil
}
