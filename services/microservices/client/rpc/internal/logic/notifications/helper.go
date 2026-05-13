package notificationslogic

import (
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func listUnreadNotificationToProto(n db.ListUnreadNotificationsRow) *client.Notification {
	return &client.Notification{
		Id:        n.ID.String(),
		UserId:    n.UserID.String(),
		Type:      n.ItemType,
		Title:     n.Title,
		Message:   n.Message,
		Read:      n.IsRead,
		CreatedAt: n.CreatedAt.Unix(),
	}
}

func listNotificationToProto(n db.ListNotificationsForUserRow) *client.Notification {
	return &client.Notification{
		Id:        n.ID.String(),
		UserId:    n.UserID.String(),
		Type:      n.ItemType,
		Title:     n.Title,
		Message:   n.Message,
		Read:      n.IsRead,
		CreatedAt: n.CreatedAt.Unix(),
	}
}
