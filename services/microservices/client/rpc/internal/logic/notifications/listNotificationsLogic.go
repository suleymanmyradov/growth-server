package notificationslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListNotificationsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListNotificationsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListNotificationsLogic {
	return &ListNotificationsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListNotificationsLogic) ListNotifications(in *client.ListNotificationsRequest) (*client.ListNotificationsResponse, error) {
	limit := int32(20)
	offset := int32(0)
	if in.Limit > 0 {
		limit = in.Limit
	}
	if in.Offset > 0 {
		offset = in.Offset
	}

	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	var notifications []*client.Notification
	var totalCount, unreadCount int64

	if in.OnlyUnread {
		dbNotifications, err := l.svcCtx.Repo.Notifications.ListUnreadNotifications(l.ctx, userID, limit, offset)
		if err != nil {
			l.Errorf("Failed to list unread notifications: %v", err)
			return nil, err
		}
		for _, n := range dbNotifications {
			notifications = append(notifications, listUnreadNotificationToProto(n))
		}
	} else {
		dbNotifications, err := l.svcCtx.Repo.Notifications.ListNotificationsForUser(l.ctx, userID, limit, offset)
		if err != nil {
			l.Errorf("Failed to list notifications: %v", err)
			return nil, err
		}
		for _, n := range dbNotifications {
			notifications = append(notifications, listNotificationToProto(n))
		}
	}

	totalCount, err = l.svcCtx.Repo.Notifications.CountNotificationsByUser(l.ctx, userID)
	if err != nil {
		l.Errorf("Failed to count notifications: %v", err)
		return nil, err
	}
	unreadCount, err = l.svcCtx.Repo.Notifications.GetUnreadCount(l.ctx, userID)
	if err != nil {
		l.Errorf("Failed to get unread count: %v", err)
		return nil, err
	}

	return &client.ListNotificationsResponse{
		Notifications: notifications,
		TotalCount:    int32(totalCount),
		UnreadCount:   int32(unreadCount),
	}, nil
}
