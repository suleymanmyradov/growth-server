package settingslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("failed to parse user ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid user ID")
	}

	settings, err := l.svcCtx.Repo.UserSettings.GetUserSettings(l.ctx, userID)
	if err != nil {
		l.Errorf("failed to get user settings: %v", err)
		return nil, status.Error(codes.NotFound, "user settings not found")
	}

	return &client.GetSettingsResponse{
		Settings: convertDbUserSettingsToPb(settings),
	}, nil
}

func convertDbUserSettingsToPb(s db.GetUserSettingsRow) *client.UserSettings {
	pb := &client.UserSettings{
		UserId:              s.UserID.String(),
		Language:            s.Language,
		Theme:               string(s.Theme),
		Timezone:            s.Timezone,
		AccountabilityStyle: string(s.AccountabilityStyle),
		OnboardingCompleted: s.OnboardingCompleted,
	}
	pb.MarketingEmails = s.EmailNotifications
	if s.CheckInTime.Valid {
		pb.CheckInTime = s.CheckInTime.Time.Format("15:04")
	}
	return pb
}
