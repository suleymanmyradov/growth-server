package settingslogic

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetSettingsLogic.GetSettings")
	defer span.End()

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("failed to parse user ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid user ID")
	}

	settings, err := l.svcCtx.Repo.UserSettings.GetUserSettings(ctx, userID)
	if err != nil {
		l.Errorf("failed to get user settings: %v", err)
		return nil, status.Error(codes.NotFound, "user settings not found")
	}

	return &client.GetSettingsResponse{
		Settings: convertDbUserSettingsToPb(settings),
	}, nil
}

func convertDbUserSettingsToPb(s db.UserSetting) *client.UserSettings {
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
		t := time.Unix(0, s.CheckInTime.Microseconds*1000)
		pb.CheckInTime = t.Format("15:04")
	}
	return pb
}
