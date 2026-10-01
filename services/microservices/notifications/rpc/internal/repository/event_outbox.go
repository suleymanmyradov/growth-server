package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"go.opentelemetry.io/otel"
)

// EventOutboxRepo wraps sqlc-generated queries for notification_event_outbox.
// Rows are written inside domain transactions; a relay drains them to the
// broker with stable event IDs (P1).
type EventOutboxRepo struct {
	db *db.Queries
}

// NewEventOutboxRepo returns a repo backed by the given sqlc Queries.
func NewEventOutboxRepo(q *db.Queries) *EventOutboxRepo {
	return &EventOutboxRepo{db: q}
}

// EnqueueEvent writes a pending event publication row.
func (r *EventOutboxRepo) EnqueueEvent(ctx context.Context, eventID uuid.UUID, eventType string, payload []byte, occurredAt time.Time) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "EventOutboxRepo.EnqueueEvent")
	defer span.End()
	return r.db.EnqueueNotificationEvent(ctx, eventID, eventType, payload, pgtype.Timestamptz{Time: occurredAt, Valid: true})
}
