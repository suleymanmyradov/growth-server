package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
)

type RecipientsRepo struct {
	db *db.Queries
}

func NewRecipientsRepo(q *db.Queries) *RecipientsRepo {
	return &RecipientsRepo{db: q}
}

func (r *RecipientsRepo) Upsert(ctx context.Context, userID uuid.UUID, email, name string, verified bool) (db.NotificationRecipient, error) {
	return r.db.UpsertNotificationRecipient(ctx, userID, email, name, verified)
}

func (r *RecipientsRepo) Get(ctx context.Context, userID uuid.UUID) (db.NotificationRecipient, error) {
	return r.db.GetNotificationRecipient(ctx, userID)
}

func (r *RecipientsRepo) Delete(ctx context.Context, userID uuid.UUID) error {
	return r.db.DeleteNotificationRecipient(ctx, userID)
}
