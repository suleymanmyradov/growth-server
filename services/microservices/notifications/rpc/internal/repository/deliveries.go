package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
)

type DeliveriesRepo struct {
	db *db.Queries
}

func NewDeliveriesRepo(q *db.Queries) *DeliveriesRepo {
	return &DeliveriesRepo{db: q}
}

func (r *DeliveriesRepo) Create(ctx context.Context, notificationID, userID uuid.UUID, channel string) (db.NotificationDelivery, error) {
	return r.db.CreateNotificationDelivery(ctx, notificationID, userID, channel)
}

func (r *DeliveriesRepo) Claim(ctx context.Context, limit int32) ([]db.NotificationDelivery, error) {
	return r.db.ClaimNotificationDeliveries(ctx, limit)
}

func (r *DeliveriesRepo) ReleaseStale(ctx context.Context, leaseMinutes int32) (int64, error) {
	return r.db.ReleaseStaleNotificationDeliveries(ctx, leaseMinutes)
}

func (r *DeliveriesRepo) MarkSent(ctx context.Context, id uuid.UUID, providerMessageID *string) error {
	return r.db.MarkNotificationDeliverySent(ctx, id, providerMessageID)
}

func (r *DeliveriesRepo) MarkSuppressed(ctx context.Context, id uuid.UUID, code, message string) error {
	return r.db.MarkNotificationDeliverySuppressed(ctx, id, &code, &message)
}

func (r *DeliveriesRepo) MarkFailed(ctx context.Context, id uuid.UUID, code, message string) error {
	return r.db.MarkNotificationDeliveryFailed(ctx, id, &code, &message)
}

func (r *DeliveriesRepo) Retry(ctx context.Context, id uuid.UUID, next time.Time, code, message string) error {
	return r.db.RetryNotificationDelivery(ctx, id, pgtype.Timestamptz{Time: next, Valid: true}, &code, &message)
}
