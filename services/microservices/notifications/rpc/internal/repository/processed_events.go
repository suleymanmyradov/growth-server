package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"go.opentelemetry.io/otel"
)

// ProcessedEventsRepo tracks which events have already been handled for
// idempotency. IsProcessed is a read-only check; Mark records an event as
// processed and should only be called after successful handling.
type ProcessedEventsRepo struct {
	db *db.Queries
}

// NewProcessedEventsRepo returns a repo backed by the given sqlc Queries.
func NewProcessedEventsRepo(q *db.Queries) *ProcessedEventsRepo {
	return &ProcessedEventsRepo{db: q}
}

// IsProcessed returns true when the event has already been recorded.
// This is a read-only SELECT and does not mutate the processed_events table.
func (r *ProcessedEventsRepo) IsProcessed(ctx context.Context, eventID uuid.UUID) bool {
	ctx, span := otel.Tracer("notifications").Start(ctx, "ProcessedEventsRepo.IsProcessed")
	defer span.End()

	processed, err := r.db.IsEventProcessed(ctx, eventID)
	if err != nil {
		return false
	}
	return processed
}

// Mark records eventID as processed. Uses ON CONFLICT DO NOTHING so it is
// safe to call more than once. Should only be called after the event has
// been successfully handled.
func (r *ProcessedEventsRepo) Mark(ctx context.Context, eventID uuid.UUID) error {
	return r.db.MarkEventProcessed(ctx, eventID)
}
