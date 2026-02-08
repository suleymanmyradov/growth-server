package notificationslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
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

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Logger.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	var notifications []*client.Notification
	var totalCount, unreadCount int64

	if in.OnlyUnread {
		dbNotifications, err := l.svcCtx.Repo.Notifications.ListUnreadNotifications(l.ctx, userID, limit, offset)
		if err != nil {
			l.Logger.Errorf("Failed to list unread notifications: %v", err)
			return nil, err
		}
		for _, n := range dbNotifications {
			notifications = append(notifications, convertDbNotificationToPb(n))
		}
	} else {
		dbNotifications, err := l.svcCtx.Repo.Notifications.ListNotificationsForUser(l.ctx, userID, limit, offset)
		if err != nil {
			l.Logger.Errorf("Failed to list notifications: %v", err)
			return nil, err
		}
		for _, n := range dbNotifications {
			notifications = append(notifications, convertDbNotificationToPb(n))
		}
	}

	totalCount, _ = l.svcCtx.Repo.Notifications.CountNotificationsByUser(l.ctx, userID)
	unreadCount, _ = l.svcCtx.Repo.Notifications.GetUnreadCount(l.ctx, userID)

	return &client.ListNotificationsResponse{
		Notifications: notifications,
		TotalCount:    int32(totalCount),
		UnreadCount:   int32(unreadCount),
	}, nil
}

func convertDbNotificationToPb(n db.Notification) *client.Notification {
	pb := &client.Notification{
		Id:        n.ID.String(),
		UserId:    n.UserID.String(),
		Type:      n.ItemType,
		Title:     n.Title,
		Message:   n.Message,
		CreatedAt: n.CreatedAt.Time.Unix(),
	}
	if n.Read.Valid {
		pb.Read = n.Read.Bool
	}
	return pb
}
