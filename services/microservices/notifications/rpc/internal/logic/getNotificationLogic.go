package logic

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetNotificationLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetNotificationLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNotificationLogic {
	return &GetNotificationLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetNotificationLogic) GetNotification(in *notifications.GetNotificationRequest) (*notifications.GetNotificationResponse, error) {
	if in == nil || in.NotificationId == "" {
		return nil, status.Error(codes.InvalidArgument, "notification ID is required")
	}

	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	notificationID, err := uuid.Parse(in.NotificationId)
	if err != nil {
		l.Errorf("failed to parse notification ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid notification ID")
	}

	notification, err := l.svcCtx.Repo.Notifications.GetNotificationByID(l.ctx, notificationID)
	if err != nil {
		l.Errorf("failed to get notification: %v", err)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "notification not found")
		}
		return nil, status.Error(codes.Internal, "failed to get notification")
	}

	if notification.UserID.String() != p.UserID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}

	return &notifications.GetNotificationResponse{
		Notification: &notifications.Notification{
			Id:        notification.ID.String(),
			UserId:    notification.UserID.String(),
			Type:      string(notification.ItemType),
			Title:     notification.Title,
			Message:   notification.Message,
			Read:      notification.IsRead,
			CreatedAt: notification.CreatedAt.Time.Unix(),
		},
	}, nil
}
