// Package deletion publishes queued user_deleted events durably. DeleteUser
// writes an auth_deletion_outbox row atomically with the user delete; this
// worker claims pending rows, republishes the event with the same event ID
// (consumers dedupe on it), and removes the row only after a successful
// publish. Rows are never evicted by age or attempt count.
package deletion

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

const (
	maxJobsPerPass = 50
	pollInterval   = 5 * time.Second
	perJobTimeout  = 10 * time.Second
)

// Store is the outbox persistence the worker needs (satisfied by *db.Queries).
type Store interface {
	ClaimAuthDeletion(context.Context) (db.ClaimAuthDeletionRow, error)
	CompleteAuthDeletion(context.Context, uuid.UUID) error
}

// Publisher publishes event envelopes (satisfied by *events.Publisher).
type Publisher interface {
	Publish(context.Context, events.Envelope) error
}

type Worker struct {
	store     Store
	publisher Publisher
}

func NewWorker(store Store, publisher Publisher) *Worker {
	return &Worker{store: store, publisher: publisher}
}

// RunOnce claims one pending outbox row, publishes its user_deleted event,
// and completes the row on success. It returns false when no row is due.
func (w *Worker) RunOnce(ctx context.Context) (bool, error) {
	if w.publisher == nil {
		return false, errors.New("deletion worker: nil publisher")
	}

	job, err := w.store.ClaimAuthDeletion(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim auth deletion: %w", err)
	}

	env, err := events.NewEnvelopeWithID(job.EventID.String(), events.TypeUserDeleted, events.UserDeleted{
		UserID: job.UserID.String(),
	})
	if err != nil {
		return false, fmt.Errorf("build user_deleted envelope: %w", err)
	}
	// Stable audit timestamp: when the deletion (and outbox row) was created,
	// not when this retry happened to run.
	env.OccurredAt = job.CreatedAt.Time

	if err := w.publisher.Publish(ctx, env); err != nil {
		return false, fmt.Errorf("publish user_deleted event: %w", err)
	}
	if err := w.store.CompleteAuthDeletion(ctx, job.EventID); err != nil {
		return true, fmt.Errorf("complete auth deletion %s: %w", job.EventID, err)
	}
	return true, nil
}

// Run polls the outbox until ctx is cancelled: immediately, then every
// pollInterval. Each pass processes at most maxJobsPerPass jobs with a
// per-job timeout; errors are logged without payloads and end the pass so
// failures never busy-loop.
func (w *Worker) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		for i := 0; i < maxJobsPerPass; i++ {
			jobCtx, cancel := context.WithTimeout(ctx, perJobTimeout)
			claimed, err := w.RunOnce(jobCtx)
			cancel()
			if err != nil {
				logx.WithContext(ctx).Errorf("deletion worker: %v", err)
				break
			}
			if !claimed {
				break
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}
