// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package settings

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientsettings "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/settings"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	notificationsClient "github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/notificationsClient"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSettingsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSettingsLogic {
	return &UpdateSettingsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateSettingsLogic) UpdateSettings(req *types.UpdateSettingsRequest) (resp *types.SettingsResponse, err error) {
	_, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return &types.SettingsResponse{Data: types.Settings{}}, nil
	}

	_, err = l.svcCtx.ClientRpc.Settings.UpdateSettings(l.ctx, &clientsettings.UpdateSettingsRequest{
		Settings: &client.UserSettings{
			Theme:               req.Theme,
			Language:            req.Language,
			Timezone:            req.Timezone,
			AccountabilityStyle: req.AccountabilityStyle,
			CheckInTime:         req.CheckInTime,
			OnboardingCompleted: req.OnboardingCompleted,
		},
	})
	if err != nil {
		return nil, err
	}

	// Forward notification preference updates to the notifications service,
	// which owns the notification_preferences table. Only update fields that
	// were explicitly provided (non-nil *bool) to avoid resetting unrelated
	// toggles when the client sends a partial update. The proto fields are
	// proto3 optional, so a nil *bool stays absent on the wire and the
	// notifications service preserves the stored value.
	if req.EmailNotifications != nil || req.PushNotifications != nil ||
		req.HabitReminders != nil || req.GoalReminders != nil {
		prefReq := &notificationsClient.UpdateNotificationPreferencesRequest{
			Preferences: &notificationsClient.NotificationPreferences{
				EmailEnabled:          req.EmailNotifications,
				PushEnabled:           req.PushNotifications,
				HabitRemindersEnabled: req.HabitReminders,
				GoalRemindersEnabled:  req.GoalReminders,
			},
		}
		if _, err := l.svcCtx.NotificationsRpc.UpdateNotificationPreferences(l.ctx, prefReq); err != nil {
			l.Errorf("UpdateSettings: failed to update notification preferences: %v", err)
			return nil, err
		}
	}

	// Return the merged settings by delegating to GetSettings (fetches from
	// both the client and notifications services).
	getLogic := NewGetSettingsLogic(l.ctx, l.svcCtx)
	return getLogic.GetSettings()
}
