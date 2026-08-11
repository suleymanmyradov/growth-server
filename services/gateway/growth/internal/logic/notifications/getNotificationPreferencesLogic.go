// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package notifications

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	notificationsClient "github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/notificationsClient"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetNotificationPreferencesLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetNotificationPreferencesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNotificationPreferencesLogic {
	return &GetNotificationPreferencesLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetNotificationPreferencesLogic) GetNotificationPreferences() (resp *types.NotificationPreferencesResponse, err error) {
	if _, ok := principal.PrincipalFrom(l.ctx); !ok {
		return &types.NotificationPreferencesResponse{
			Preferences: types.NotificationPreferences{},
		}, nil
	}

	rpcResp, err := l.svcCtx.NotificationsRpc.GetNotificationPreferences(l.ctx, &notificationsClient.GetNotificationPreferencesRequest{})
	if err != nil {
		return nil, err
	}

	return &types.NotificationPreferencesResponse{
		Preferences: notificationPreferencesFromRPC(rpcResp.Preferences),
	}, nil
}

func notificationPreferencesFromRPC(p *notificationsClient.NotificationPreferences) types.NotificationPreferences {
	if p == nil {
		return types.NotificationPreferences{}
	}
	return types.NotificationPreferences{
		EmailEnabled:          p.EmailEnabled,
		PushEnabled:           p.PushEnabled,
		HabitRemindersEnabled: p.HabitRemindersEnabled,
		GoalRemindersEnabled:  p.GoalRemindersEnabled,
		StreakWarningsEnabled: p.StreakWarningsEnabled,
		SundayReviewEnabled:   p.SundayReviewEnabled,
	}
}
