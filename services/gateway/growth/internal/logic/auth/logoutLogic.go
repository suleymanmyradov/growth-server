// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package auth

import (
	"context"
	"strings"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"
	notificationsClient "github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/notificationsClient"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogoutLogic {
	return &LogoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LogoutLogic) Logout(req *types.LogoutRequest) (resp *types.EmptyResponse, err error) {
	authHeader := l.ctx.Value("Authorization")
	if authHeader == nil {
		return &types.EmptyResponse{}, nil
	}

	authHeaderValue, ok := authHeader.(string)
	if !ok {
		return &types.EmptyResponse{}, nil
	}
	token := strings.TrimPrefix(authHeaderValue, "Bearer ")

	_, err = l.svcCtx.AuthRpc.Logout(l.ctx, &authservice.LogoutRequest{
		AccessToken:  token,
		RefreshToken: req.RefreshToken,
	})
	if err != nil {
		return nil, err
	}

	// Best-effort device unregistration: if the client sends X-Device-Id on
	// logout, ask the notifications service to unregister that installation so
	// the device stops receiving push notifications for this user. This is
	// gateway composition (calling two RPCs) — the auth and notifications RPCs
	// never call each other. A failure here is logged but does NOT fail logout:
	// the session is already revoked, so the security property holds regardless
	// of whether the device token is cleaned up immediately.
	if req.DeviceId != "" {
		_, derr := l.svcCtx.NotificationsRpc.UnregisterDevice(l.ctx, &notificationsClient.UnregisterDeviceRequest{
			InstallationId: req.DeviceId,
		})
		if derr != nil {
			logx.WithContext(l.ctx).Errorf("logout: best-effort device unregistration failed for installation %s: %v", req.DeviceId, derr)
		}
	}

	return &types.EmptyResponse{}, nil
}
