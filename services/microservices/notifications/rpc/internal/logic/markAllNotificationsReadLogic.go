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

type MarkAllNotificationsReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMarkAllNotificationsReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkAllNotificationsReadLogic {
	return &MarkAllNotificationsReadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *MarkAllNotificationsReadLogic) MarkAllNotificationsRead(in *notifications.MarkAllNotificationsReadRequest) (*notifications.MarkAllNotificationsReadResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "MarkAllNotificationsReadLogic.MarkAllNotificationsRead")
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

	err = l.svcCtx.Repo.Notifications.MarkAllNotificationsRead(ctx, userID)
	if err != nil {
		logx.WithContext(ctx).Errorf("Failed to mark all notifications as read: %v", err)
		return nil, status.Error(codes.Internal, "failed to mark all notifications as read")
	}

	return &notifications.MarkAllNotificationsReadResponse{
		Success: true,
	}, nil
}
