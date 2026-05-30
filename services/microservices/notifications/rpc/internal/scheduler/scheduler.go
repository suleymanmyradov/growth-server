package scheduler

import (
	"context"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

// Repo is the scheduler's storage interface, declared here so the consumer
// package owns the abstraction.
type Repo interface {
	ClaimDueReminders(ctx context.Context, limit int32) ([]db.ReminderQueue, error)
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

// Scheduler claims due reminders from the database and publishes ReminderDue
// events to Kafka on each tick.
type Scheduler struct {
	repo     Repo
	pub      Publisher
	clock    Clock
	interval time.Duration
	batch    int32
}

// NewScheduler creates a Scheduler with the given dependencies and options.
func NewScheduler(repo Repo, pub Publisher, clock Clock, opts ...Option) *Scheduler {
	s := &Scheduler{
		repo:     repo,
		pub:      pub,
		clock:    clock,
		interval: 60 * time.Second,
		batch:    100,
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
			ScheduledAt: r.ScheduledAt.Format(time.RFC3339),
			Metadata:    string(r.Metadata),
		})
		if err != nil {
			logx.WithContext(ctx).Errorf("envelope for reminder %s: %v", r.ID, err)
			continue
		}

		if err := s.pub.Publish(ctx, env); err != nil {
			logx.WithContext(ctx).Errorf("publish reminder %s: %v", r.ID, err)
			continue
		}

		logx.WithContext(ctx).Infof("published reminder %s type=%s", r.ID, r.Type)
	}
}
