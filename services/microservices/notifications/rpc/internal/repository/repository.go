package repository

import (
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
)

type Repository struct {
	Notifications   *NotificationsRepo
	Reminders       *RemindersRepo
	ProcessedEvents *ProcessedEventsRepo
}

func NewRepository(q *db.Queries) *Repository {
	return &Repository{
		Notifications:   NewNotificationsRepo(q),
		Reminders:       NewRemindersRepo(q),
		ProcessedEvents: NewProcessedEventsRepo(q),
	}
}
