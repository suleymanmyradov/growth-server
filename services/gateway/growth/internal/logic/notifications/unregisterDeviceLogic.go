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

type UnregisterDeviceLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnregisterDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnregisterDeviceLogic {
	return &UnregisterDeviceLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UnregisterDeviceLogic) UnregisterDevice(req *types.UnregisterDeviceRequest) (*types.EmptyResponse, error) {
	if req == nil || req.InstallationId == "" {
		return nil, status.Error(codes.InvalidArgument, "installationId is required")
	}

	if _, ok := principal.PrincipalFrom(l.ctx); !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	_, err := l.svcCtx.NotificationsRpc.UnregisterDevice(l.ctx, &notificationsClient.UnregisterDeviceRequest{
		InstallationId: req.InstallationId,
	})
	if err != nil {
		return nil, err
	}

	return &types.EmptyResponse{}, nil
}
