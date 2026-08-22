package repository

import (
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
)

type Repository struct {
	Notifications   *NotificationsRepo
	Reminders       *RemindersRepo
	ProcessedEvents *ProcessedEventsRepo
	ReminderState   *ReminderStateRepo
	Preferences     *PreferencesRepo
	Devices         *DevicesRepo
	PushTickets     *PushTicketsRepo
	Deliveries      *DeliveriesRepo
	Recipients      *RecipientsRepo
	HabitState      *HabitStateRepo
}

func NewRepository(q *db.Queries) *Repository {
	return &Repository{
		Notifications:   NewNotificationsRepo(q),
		Reminders:       NewRemindersRepo(q),
		ProcessedEvents: NewProcessedEventsRepo(q),
		ReminderState:   NewReminderStateRepo(q),
		Preferences:     NewPreferencesRepo(q),
		Devices:         NewDevicesRepo(q),
		PushTickets:     NewPushTicketsRepo(q),
		Deliveries:      NewDeliveriesRepo(q),
		Recipients:      NewRecipientsRepo(q),
		HabitState:      NewHabitStateRepo(q),
	}
}

// NewRepositoryFromTx builds a Repository backed by the given transaction.
// All repos in the returned Repository share the same tx, so operations on
// them are atomic when the tx is committed or rolled back.
func NewRepositoryFromTx(tx pgx.Tx) *Repository {
	return NewRepository(db.New(tx))
}
