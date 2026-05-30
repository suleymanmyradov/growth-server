package logic

import (
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"
)

func listUnreadNotificationToProto(n db.ListUnreadNotificationsRow) *notifications.Notification {
	return &notifications.Notification{
		Id:        n.ID.String(),
		UserId:    n.UserID.String(),
		Type:      string(n.ItemType),
		Title:     n.Title,
		Message:   n.Message,
		Read:      n.IsRead,
		CreatedAt: n.CreatedAt.Time.Unix(),
	}
}

func listNotificationToProto(n db.ListNotificationsForUserRow) *notifications.Notification {
	return &notifications.Notification{
		Id:        n.ID.String(),
		UserId:    n.UserID.String(),
		Type:      string(n.ItemType),
		Title:     n.Title,
		Message:   n.Message,
		Read:      n.IsRead,
		CreatedAt: n.CreatedAt.Time.Unix(),
	}
}
