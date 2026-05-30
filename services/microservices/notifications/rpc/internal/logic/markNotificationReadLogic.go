package logic

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type MarkNotificationReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMarkNotificationReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkNotificationReadLogic {
	return &MarkNotificationReadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *MarkNotificationReadLogic) MarkNotificationRead(in *notifications.MarkNotificationReadRequest) (*notifications.MarkNotificationReadResponse, error) {
	if in == nil || in.NotificationId == "" {
		return nil, status.Error(codes.InvalidArgument, "notification ID is required")
	}

	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	notificationID, err := uuid.Parse(in.NotificationId)
	if err != nil {
		l.Errorf("Invalid notification ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid notification ID")
	}

	notification, err := l.svcCtx.Repo.Notifications.GetNotificationByID(l.ctx, notificationID)
	if err != nil {
		l.Errorf("Failed to get notification: %v", err)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "notification not found")
		}
		return nil, status.Error(codes.Internal, "failed to get notification")
	}

	if notification.UserID.String() != p.UserID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}

	_, err = l.svcCtx.Repo.Notifications.MarkNotificationRead(l.ctx, notificationID)
	if err != nil {
		l.Errorf("Failed to mark notification as read: %v", err)
		return nil, status.Error(codes.Internal, "failed to mark notification as read")
	}

	return &notifications.MarkNotificationReadResponse{
		Success: true,
	}, nil
}
