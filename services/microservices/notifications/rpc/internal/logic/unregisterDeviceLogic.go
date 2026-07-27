package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type UnregisterDeviceLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUnregisterDeviceLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnregisterDeviceLogic {
	return &UnregisterDeviceLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// UnregisterDevice removes a device registration for the authenticated user.
// The user_id scoping prevents a user from deleting another user's device.
func (l *UnregisterDeviceLogic) UnregisterDevice(in *notifications.UnregisterDeviceRequest) (*notifications.EmptyResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UnregisterDeviceLogic.UnregisterDevice")
	defer span.End()

	if in == nil || in.InstallationId == "" {
		return nil, status.Error(codes.InvalidArgument, "installationId is required")
	}

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("UnregisterDevice: invalid user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	if err := l.svcCtx.Repo.Devices.DeleteDevice(ctx, in.InstallationId, userID); err != nil {
		logx.WithContext(ctx).Errorf("UnregisterDevice: delete failed: %v", err)
		return nil, status.Error(codes.Internal, "failed to unregister device")
	}

	return &notifications.EmptyResponse{}, nil
}
