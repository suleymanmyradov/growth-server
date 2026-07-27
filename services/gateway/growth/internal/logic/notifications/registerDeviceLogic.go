// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package notifications

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	notificationsClient "github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/notificationsClient"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type RegisterDeviceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewRegisterDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *RegisterDeviceLogic {
	return &RegisterDeviceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *RegisterDeviceLogic) RegisterDevice(req *types.RegisterDeviceRequest) (*types.EmptyResponse, error) {
	if req == nil || req.InstallationId == "" {
		return nil, status.Error(codes.InvalidArgument, "installationId is required")
	}
	if req.PushToken == "" {
		return nil, status.Error(codes.InvalidArgument, "pushToken is required")
	}

	// Principal presence is enforced by Auth middleware; this is a defensive
	// check so an unauthenticated request never reaches the RPC.
	if _, ok := principal.PrincipalFrom(l.ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	_, err := l.svcCtx.NotificationsRpc.RegisterDevice(l.ctx, &notificationsClient.RegisterDeviceRequest{
		InstallationId: req.InstallationId,
		PushToken:      req.PushToken,
		Provider:       req.Provider,
		Platform:       req.Platform,
		AppId:          req.AppId,
		Environment:    req.Environment,
		AppVersion:     req.AppVersion,
		OsVersion:      req.OsVersion,
		Locale:         req.Locale,
		Timezone:       req.Timezone,
	})
	if err != nil {
		return nil, err
	}

	return &types.EmptyResponse{}, nil
}
