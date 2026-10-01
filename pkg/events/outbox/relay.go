// Package outbox implements the relay half of the transactional outbox
// pattern. Services insert an outbox row inside the same transaction as their
// domain write (Enqueue/EnqueueParams), then a Relay claims due rows,
// republishes the stored payload under the stored event ID, and deletes the
// row only after a successful publish. The same event ID is republished on
// every attempt, so consumers' processed_events dedup makes at-least-once
// delivery safe.
package outbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/zeromicro/go-zero/core/logx"
)

// Row is a claimed outbox record. Payload is the raw envelope payload JSON —
// the relay republishes it verbatim inside a reconstructed envelope.
type Row struct {
	EventID    uuid.UUID
	EventType  string
	Payload    []byte
	OccurredAt time.Time
}

// Store is the outbox persistence the relay needs. Claim must atomically
// lease one due row (SKIP LOCKED) or return pgx.ErrNoRows when none are due;
// Complete deletes the row after a successful publish.
type Store interface {
	Claim(ctx context.Context) (Row, error)
	Complete(ctx context.Context, eventID uuid.UUID) error
}

// Publisher publishes event envelopes (satisfied by *events.Publisher).
type Publisher interface {
	Publish(ctx context.Context, env events.Envelope) error
}

const (
	defaultMaxJobsPerPass = 50
	defaultPollInterval   = 2 * time.Second
	defaultPerJobTimeout  = 10 * time.Second
)

// Relay drains one outbox table into the broker. Rows are retried forever —
// there is no eviction by age or attempt count — so a broker outage pauses
// delivery instead of losing events.
type Relay struct {
	name          string
	store         Store
	publisher     Publisher
	pollInterval  time.Duration
	perJobTimeout time.Duration
	maxJobs       int
}

// NewRelay builds a relay. name is used only for log lines (e.g. "client").
func NewRelay(name string, store Store, publisher Publisher) *Relay {
	return &Relay{
		name:          name,
		store:         store,
		publisher:     publisher,
		pollInterval:  defaultPollInterval,
		perJobTimeout: defaultPerJobTimeout,
		maxJobs:       defaultMaxJobsPerPass,
	}
}

// EnqueueParams converts an Envelope into the column values every outbox
// insert uses: (event_id, event_type, payload, occurred_at). Build the
// envelope with events.NewEnvelope (or NewEnvelopeWithID for deterministic
// dedup) and pass these into the service's sqlc Enqueue query inside the
// domain transaction.
func EnqueueParams(env events.Envelope) (eventID uuid.UUID, eventType string, payload []byte, occurredAt pgtype.Timestamptz, err error) {
	eventID, err = uuid.Parse(env.EventID)
	if err != nil {
		return uuid.Nil, "", nil, pgtype.Timestamptz{}, fmt.Errorf("outbox enqueue: invalid event ID %q: %w", env.EventID, err)
	}
	return eventID, env.EventType, env.Payload, pgtype.Timestamptz{Time: env.OccurredAt, Valid: true}, nil
}

// RunOnce claims one due outbox row, republishes its event, and deletes the
// row on success. Returns false when no row is due.
func (r *Relay) RunOnce(ctx context.Context) (bool, error) {
	if r.publisher == nil {
		return false, errors.New("outbox relay: nil publisher")
	}
	row, err := r.store.Claim(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("%s outbox claim: %w", r.name, err)
	}

	env := events.Envelope{
		EventID:    row.EventID.String(),
		EventType:  row.EventType,
		Version:    1,
		OccurredAt: row.OccurredAt,
		Payload:    json.RawMessage(row.Payload),
	}
	if err := r.publisher.Publish(ctx, env); err != nil {
		return false, fmt.Errorf("%s outbox publish %s: %w", r.name, row.EventID, err)
	}
	if err := r.store.Complete(ctx, row.EventID); err != nil {
		// Row stays claimable until the lease expires; the republished event
		// is a duplicate ID which consumers dedupe.
		return true, fmt.Errorf("%s outbox complete %s: %w", r.name, row.EventID, err)
	}
	return true, nil
}

// Run polls the outbox until ctx is cancelled: immediately, then every
// pollInterval. Each pass processes at most maxJobs rows with a per-row
// timeout; a failed row ends the pass so broker outages never busy-loop.
func (r *Relay) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		for i := 0; i < r.maxJobs; i++ {
			jobCtx, cancel := context.WithTimeout(ctx, r.perJobTimeout)
			claimed, err := r.RunOnce(jobCtx)
			cancel()
			if err != nil {
				logx.WithContext(ctx).Errorf("outbox relay %s: %v", r.name, err)
				break
			}
			if !claimed {
				break
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(r.pollInterval):
		}
	}
}
