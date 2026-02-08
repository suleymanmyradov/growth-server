package settingslogic

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateSettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateSettingsLogic {
	return &UpdateSettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateSettingsLogic) UpdateSettings(in *client.UpdateSettingsRequest) (*client.UpdateSettingsResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Logger.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	params := db.UpdateUserSettingsParams{
		UserID: userID,
	}

	if in.Settings != nil {
		if in.Settings.Theme != "" {
			params.Theme = in.Settings.Theme
		}
		if in.Settings.Language != "" {
			params.Language = in.Settings.Language
		}
		if in.Settings.Timezone != "" {
			params.Timezone = in.Settings.Timezone
		}
		params.EmailNotifications = sql.NullBool{Bool: in.Settings.MarketingEmails, Valid: true}
		params.PushNotifications = sql.NullBool{Bool: true, Valid: true}
		params.HabitReminders = sql.NullBool{Bool: true, Valid: true}
		params.GoalReminders = sql.NullBool{Bool: true, Valid: true}
	}

	_, err = l.svcCtx.Repo.UserSettings.UpdateUserSettings(l.ctx, params)
	if err != nil {
		l.Logger.Errorf("Failed to update user settings: %v", err)
		return nil, err
	}

	return &client.UpdateSettingsResponse{
		Success: true,
	}, nil
}
