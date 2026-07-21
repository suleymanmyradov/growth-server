// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package notifications

import (
	"context"
	"fmt"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	notificationsClient "github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/notificationsClient"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateNotificationPreferencesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateNotificationPreferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateNotificationPreferencesLogic {
	return &UpdateNotificationPreferencesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateNotificationPreferencesLogic) UpdateNotificationPreferences(req *types.UpdateNotificationPreferencesRequest) (resp *types.NotificationPreferencesResponse, err error) {
	if _, ok := principal.PrincipalFrom(l.ctx); !ok {
		return nil, fmt.Errorf("unauthenticated")
	}

	rpcResp, err := l.svcCtx.NotificationsRpc.UpdateNotificationPreferences(l.ctx, &notificationsClient.UpdateNotificationPreferencesRequest{		Preferences: &notificationsClient.NotificationPreferences{
			EmailEnabled:          req.Preferences.EmailEnabled,
			PushEnabled:           req.Preferences.PushEnabled,
			HabitRemindersEnabled: req.Preferences.HabitRemindersEnabled,
			GoalRemindersEnabled:  req.Preferences.GoalRemindersEnabled,
		},
	})
	if err != nil {
		return nil, err
	}

	return &types.NotificationPreferencesResponse{
		Preferences: notificationPreferencesFromRPC(rpcResp.Preferences),
	}, nil
}
