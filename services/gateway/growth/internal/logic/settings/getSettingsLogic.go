// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package settings

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientsettings "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/settings"
	notificationsClient "github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/notificationsClient"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSettingsLogic {
	return &GetSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetSettingsLogic) GetSettings() (resp *types.SettingsResponse, err error) {
	_, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return &types.SettingsResponse{Data: types.Settings{}}, nil
	}

	rpcResp, err := l.svcCtx.ClientRpc.Settings.GetSettings(l.ctx, &clientsettings.GetSettingsRequest{})
	if err != nil {
		return nil, err
	}

	// Fetch notification preferences from the notifications service (owns the
	// notification_preferences table). The gateway merges the two services'
	// settings into a single Settings response — no RPC-to-RPC calls.
	var emailNotif, pushNotif, habitRem, goalRem bool
	if prefResp, err := l.svcCtx.NotificationsRpc.GetNotificationPreferences(l.ctx, &notificationsClient.GetNotificationPreferencesRequest{}); err != nil {
		l.Errorf("GetSettings: failed to fetch notification preferences: %v", err)
		// Non-fatal: default all notification flags to false.
	} else if prefResp.Preferences != nil {
		emailNotif = prefResp.Preferences.EmailEnabled
		pushNotif = prefResp.Preferences.PushEnabled
		habitRem = prefResp.Preferences.HabitRemindersEnabled
		goalRem = prefResp.Preferences.GoalRemindersEnabled
	}

	return &types.SettingsResponse{
		Data: types.Settings{
			Id:                  rpcResp.Settings.UserId,
			Theme:               rpcResp.Settings.Theme,
			Language:            rpcResp.Settings.Language,
			Timezone:            rpcResp.Settings.Timezone,
			EmailNotifications:  emailNotif,
			PushNotifications:   pushNotif,
			HabitReminders:      habitRem,
			GoalReminders:       goalRem,
			AccountabilityStyle: rpcResp.Settings.AccountabilityStyle,
			CheckInTime:         rpcResp.Settings.CheckInTime,
			OnboardingCompleted: rpcResp.Settings.OnboardingCompleted,
			UserId:              rpcResp.Settings.UserId,
		},
	}, nil
}
