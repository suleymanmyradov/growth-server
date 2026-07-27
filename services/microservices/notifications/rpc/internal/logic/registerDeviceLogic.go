package logic

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type RegisterDeviceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewRegisterDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterDeviceLogic {
	return &RegisterDeviceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// RegisterDevice upserts a push device registration for the authenticated user.
// The user is derived from the principal in the RPC context; installationId
// comes from the gateway path. See docs/push-notifications-design.md.
func (l *RegisterDeviceLogic) RegisterDevice(in *notifications.RegisterDeviceRequest) (*notifications.EmptyResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "RegisterDeviceLogic.RegisterDevice")
	defer span.End()

	if in == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	if in.InstallationId == "" {
		return nil, status.Error(codes.InvalidArgument, "installationId is required")
	}
	if in.PushToken == "" {
		return nil, status.Error(codes.InvalidArgument, "pushToken is required")
	}
	provider := strings.ToLower(strings.TrimSpace(in.Provider))
	if provider == "" {
		provider = "expo"
	}
	if provider != "expo" && provider != "fcm" && provider != "apns" {
		return nil, status.Error(codes.InvalidArgument, "provider must be expo, fcm, or apns")
	}
	platform := strings.ToLower(strings.TrimSpace(in.Platform))
	if platform != "ios" && platform != "android" {
		return nil, status.Error(codes.InvalidArgument, "platform must be ios or android")
	}
	environment := strings.ToLower(strings.TrimSpace(in.Environment))
	if environment == "" {
		environment = "production"
	}
	if environment != "development" && environment != "preview" && environment != "production" {
		return nil, status.Error(codes.InvalidArgument, "environment must be development, preview, or production")
	}

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("RegisterDevice: invalid user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	_, err = l.svcCtx.Repo.Devices.UpsertDevice(ctx, db.UpsertDeviceParams{
		UserID:         userID,
		InstallationID: in.InstallationId,
		Provider:       provider,
		PushToken:      in.PushToken,
		Platform:       platform,
		AppID:          nullableString(in.AppId),
		Environment:    environment,
		AppVersion:     nullableString(in.AppVersion),
		OsVersion:      nullableString(in.OsVersion),
		Locale:         nullableString(in.Locale),
		Timezone:       nullableString(in.Timezone),
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("RegisterDevice: upsert failed: %v", err)
		return nil, status.Error(codes.Internal, "failed to register device")
	}

	return &notifications.EmptyResponse{}, nil
}

// nullableString returns a *string for the trimmed value, or nil when empty,
// matching the nullable varchar columns in notification_devices.
func nullableString(s string) *string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return &s
}
