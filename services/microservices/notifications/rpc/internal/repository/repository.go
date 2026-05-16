package repository

import (
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
)

type Repository struct {
	Notifications *NotificationsRepo
}

func NewRepository(db *db.Queries) *Repository {
	return &Repository{
		Notifications: NewNotificationsRepo(db),
	}
}
