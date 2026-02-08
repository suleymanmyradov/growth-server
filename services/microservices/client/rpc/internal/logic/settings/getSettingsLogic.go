package settingslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetSettingsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetSettingsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetSettingsLogic {
	return &GetSettingsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetSettingsLogic) GetSettings(in *client.GetSettingsRequest) (*client.GetSettingsResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Logger.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	settings, err := l.svcCtx.Repo.UserSettings.GetUserSettings(l.ctx, userID)
	if err != nil {
		l.Logger.Errorf("Failed to get user settings: %v", err)
		return nil, err
	}

	return &client.GetSettingsResponse{
		Settings: convertDbUserSettingsToPb(settings),
	}, nil
}

func convertDbUserSettingsToPb(s db.UserSetting) *client.UserSettings {
	pb := &client.UserSettings{
		UserId:   s.UserID.String(),
		Language: s.Language,
		Theme:    s.Theme,
		Timezone: s.Timezone,
	}
	if s.EmailNotifications.Valid {
		pb.MarketingEmails = s.EmailNotifications.Bool
	}
	return pb
}
