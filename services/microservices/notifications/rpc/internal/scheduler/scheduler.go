package scheduler

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

// Repo is the scheduler's storage interface, declared here so the consumer
// package owns the abstraction.
type Repo interface {
	ClaimDueReminders(ctx context.Context, limit int32) ([]db.ClaimDueRemindersRow, error)
	MarkSent(ctx context.Context, id uuid.UUID) (db.MarkReminderSentRow, error)
	ReleaseStaleClaims(ctx context.Context, leaseMinutes int32) (int64, error)
}

// Publisher is the scheduler's publishing interface.
type Publisher interface {
	Publish(ctx context.Context, env events.Envelope) error
}

// Clock abstracts time.Now for testability.
type Clock interface {
	Now() time.Time
}

// Option configures a Scheduler via functional options.
type Option func(*Scheduler)

// WithInterval sets the tick interval. Defaults to 60s.
func WithInterval(d time.Duration) Option {
	return func(s *Scheduler) { s.interval = d }
}

// WithBatch sets the claim batch size. Defaults to 100.
func WithBatch(n int32) Option {
	return func(s *Scheduler) { s.batch = n }
}

// WithLease sets the claim lease duration in minutes. A reminder whose
// claimed_at is older than the lease (and sent_at is still NULL) is
// considered stale and re-claimed. Defaults to 5 minutes.
func WithLease(minutes int32) Option {
	return func(s *Scheduler) { s.leaseMinutes = minutes }
}

// Scheduler claims due reminders from the database and publishes ReminderDue
// events to Kafka on each tick. It uses a claim/lease/ack model: reminders
// are claimed (leased) before publishing and only marked sent after
// successful publication, so a publish failure or scheduler crash does not
// lose reminders — the lease expires and the reminder is re-claimed.
type Scheduler struct {
	repo         Repo
	pub          Publisher
	clock        Clock
	interval     time.Duration
	batch        int32
	leaseMinutes int32
}

// NewScheduler creates a Scheduler with the given dependencies and options.
func NewScheduler(repo Repo, pub Publisher, clock Clock, opts ...Option) *Scheduler {
	s := &Scheduler{
		repo:         repo,
		pub:          pub,
		clock:        clock,
		interval:     60 * time.Second,
		batch:        100,
		leaseMinutes: 5,
	}
	for _, o := range opts {
		o(s)
	}
	return s
}

// Run starts the scheduler loop. It ticks at the configured interval, claiming
// due reminders and publishing events. Blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	// Release stale claims from crashed scheduler instances so their
	// reminders become claimable again.
	if released, err := s.repo.ReleaseStaleClaims(ctx, s.leaseMinutes); err != nil {
		logx.WithContext(ctx).Errorf("release stale claims: %v", err)
	} else if released > 0 {
		logx.WithContext(ctx).Infof("released %d stale reminder claims", released)
	}

	reminders, err := s.repo.ClaimDueReminders(ctx, s.batch)
	if err != nil {
		logx.WithContext(ctx).Errorf("claim due reminders: %v", err)
		return
	}

	for _, r := range reminders {
		env, err := events.NewEnvelope(events.TypeReminderDue, events.ReminderDue{
			ReminderID:  r.ID.String(),
			UserID:      r.UserID.String(),
			Type:        string(r.Type),
			ScheduledAt: r.ScheduledAt.Time.Format(time.RFC3339),
			Metadata:    string(r.Metadata),
		})
		if err != nil {
			logx.WithContext(ctx).Errorf("envelope for reminder %s: %v", r.ID, err)
			// Cannot build envelope — leave the lease; it will expire and
			// the reminder will be re-claimed. Do not ack.
			continue
		}

		if err := s.pub.Publish(ctx, env); err != nil {
			// Publish failed — do NOT ack. The lease will expire and the
			// reminder will be re-claimed on a future tick. This is the
			// at-least-once delivery guarantee.
			logx.WithContext(ctx).Errorf("publish reminder %s: %v (will retry after lease expires)", r.ID, err)
			continue
		}

		// Ack: mark the reminder as sent only after successful publication.
		if _, err := s.repo.MarkSent(ctx, r.ID); err != nil {
			// Marking sent failed — the reminder is published but not acked.
			// The lease will expire and the reminder may be re-claimed and
			// re-published (at-least-once). The consumer must be idempotent.
			logx.WithContext(ctx).Errorf("ack reminder %s (published but not marked sent): %v", r.ID, err)
			continue
		}

		logx.WithContext(ctx).Infof("published reminder %s type=%s", r.ID, r.Type)
	}
}
