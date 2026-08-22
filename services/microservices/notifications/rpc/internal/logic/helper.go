package logic

import (
	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/pb/notifications"
)

func listUnreadNotificationToProto(n db.ListUnreadNotificationsRow) *notifications.Notification {
	return &notifications.Notification{
		Id:          n.ID.String(),
		UserId:      n.UserID.String(),
		Type:        n.Type,
		Title:       n.Title,
		Message:     n.Message,
		Read:        n.IsRead,
		CreatedAt:   n.CreatedAt.Time.Unix(),
		Destination: stringValue(n.Destination),
		ResourceId:  nullUUIDValue(n.ResourceID),
		Metadata:    string(n.Metadata),
	}
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func nullUUIDValue(value uuid.NullUUID) string {
	if !value.Valid {
		return ""
	}
	return value.UUID.String()
}

func listNotificationToProto(n db.ListNotificationsForUserRow) *notifications.Notification {
	return &notifications.Notification{
		Id:          n.ID.String(),
		UserId:      n.UserID.String(),
		Type:        n.Type,
		Title:       n.Title,
		Message:     n.Message,
		Read:        n.IsRead,
		CreatedAt:   n.CreatedAt.Time.Unix(),
		Destination: stringValue(n.Destination),
		ResourceId:  nullUUIDValue(n.ResourceID),
		Metadata:    string(n.Metadata),
	}
}
