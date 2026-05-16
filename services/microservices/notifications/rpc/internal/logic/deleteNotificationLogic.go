package logic

import (
	"context"
	"database/sql"
	"errors"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteNotificationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteNotificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteNotificationLogic {
	return &DeleteNotificationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteNotificationLogic) DeleteNotification(in *notifications.DeleteNotificationRequest) (*notifications.DeleteNotificationResponse, error) {
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
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "notification not found")
		}
		return nil, status.Error(codes.Internal, "failed to get notification")
	}

	if notification.UserID.String() != p.UserID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}

	err = l.svcCtx.Repo.Notifications.DeleteNotification(l.ctx, notificationID)
	if err != nil {
		l.Errorf("Failed to delete notification: %v", err)
		return nil, status.Error(codes.Internal, "failed to delete notification")
	}

	return &notifications.DeleteNotificationResponse{
		Success: true,
	}, nil
}
