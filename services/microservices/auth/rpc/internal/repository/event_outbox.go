package repository

import (
	"context"
	"fmt"

	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/events/outbox"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
)

// IEventOutbox writes lifecycle events into auth_event_outbox inside the
// caller's transaction (txRepo). The outbox relay republishes them to the
// broker with their stable event IDs; rows are deleted only after a
// successful publish, so events can no longer be lost between the domain
// commit and the broker write.
type IEventOutbox interface {
	Enqueue(ctx context.Context, env events.Envelope) error
}

type eventOutboxRepo struct {
	q *db.Queries
}

// NewEventOutboxRepo creates an outbox repo over the given queries (pool or tx).
func NewEventOutboxRepo(q *db.Queries) IEventOutbox {
	return &eventOutboxRepo{q: q}
}

func (r *eventOutboxRepo) Enqueue(ctx context.Context, env events.Envelope) error {
	eventID, eventType, payload, occurredAt, err := outbox.EnqueueParams(env)
	if err != nil {
		return err
	}
	if err := r.q.EnqueueAuthEvent(ctx, eventID, eventType, payload, occurredAt); err != nil {
		return fmt.Errorf("enqueue auth event %s: %w", env.EventID, err)
	}
	return nil
}
