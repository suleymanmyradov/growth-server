package logic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetUnreadCountLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUnreadCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUnreadCountLogic {
	return &GetUnreadCountLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUnreadCountLogic) GetUnreadCount(in *notifications.GetUnreadCountRequest) (*notifications.GetUnreadCountResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetUnreadCountLogic.GetUnreadCount")
	defer span.End()

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	count, err := l.svcCtx.Repo.Notifications.GetUnreadCount(ctx, userID)
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to count unread notifications: %v", err)
		return nil, status.Error(codes.Internal, "failed to count unread notifications")
	}

	return &notifications.GetUnreadCountResponse{
		Count: int32(count),
	}, nil
}
