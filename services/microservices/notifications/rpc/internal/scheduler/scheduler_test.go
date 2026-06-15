package scheduler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
)

// ---- fakes ----

type fakeRepo struct {
	claimed []db.Reminder
	err     error
}

func (f *fakeRepo) ClaimDueReminders(_ context.Context, _ int32) ([]db.Reminder, error) {
	return f.claimed, f.err
}

type fakePub struct {
	published []events.Envelope
	err       error
}

func (f *fakePub) Publish(_ context.Context, env events.Envelope) error {
	f.published = append(f.published, env)
	return f.err
}

type fakeSchedClock struct {
	t time.Time
}

func (f fakeSchedClock) Now() time.Time { return f.t }

// ---- tests ----

func TestScheduler_Tick_Empty(t *testing.T) {
	repo := &fakeRepo{}
	pub := &fakePub{}
	s := NewScheduler(repo, pub, fakeSchedClock{t: time.Now()}, WithInterval(time.Hour))

	s.tick(context.Background())
	if len(pub.published) != 0 {
		t.Fatalf("expected 0 publishes, got %d", len(pub.published))
	}
}

func TestScheduler_Tick_ClaimAndPublish(t *testing.T) {
	uid := uuid.New()
	rid := uuid.New()
	repo := &fakeRepo{
		claimed: []db.Reminder{
			{ID: rid, UserID: uid, Type: "habit_reminder", ScheduledAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
		},
	}
	pub := &fakePub{}
	s := NewScheduler(repo, pub, fakeSchedClock{t: time.Now()})

	s.tick(context.Background())
	if len(pub.published) != 1 {
		t.Fatalf("expected 1 publish, got %d", len(pub.published))
	}

	var due events.ReminderDue
	if err := json.Unmarshal(pub.published[0].Payload, &due); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if due.ReminderID != rid.String() {
		t.Errorf("expected reminderID %s, got %s", rid, due.ReminderID)
	}
	if due.Type != "habit_reminder" {
		t.Errorf("expected type habit_reminder, got %s", due.Type)
	}
}

func TestScheduler_Tick_PublishError(t *testing.T) {
	uid := uuid.New()
	repo := &fakeRepo{
		claimed: []db.Reminder{
			{ID: uuid.New(), UserID: uid, Type: "habit_reminder", ScheduledAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
		},
	}
	pub := &fakePub{err: context.DeadlineExceeded}
	s := NewScheduler(repo, pub, fakeSchedClock{t: time.Now()})

	// Should not panic, just log.
	s.tick(context.Background())
}

func TestScheduler_Tick_ClaimError(t *testing.T) {
	repo := &fakeRepo{err: context.Canceled}
	pub := &fakePub{}
	s := NewScheduler(repo, pub, fakeSchedClock{t: time.Now()})

	// Should not panic, just log.
	s.tick(context.Background())
	if len(pub.published) != 0 {
		t.Fatalf("expected 0 publishes on claim error, got %d", len(pub.published))
	}
}

func TestScheduler_MultipleReminders(t *testing.T) {
	uid := uuid.New()
	repo := &fakeRepo{
		claimed: []db.Reminder{
			{ID: uuid.New(), UserID: uid, Type: "habit_reminder", ScheduledAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
			{ID: uuid.New(), UserID: uid, Type: "weekly_review", ScheduledAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
		},
	}
	pub := &fakePub{}
	s := NewScheduler(repo, pub, fakeSchedClock{t: time.Now()})

	s.tick(context.Background())
	if len(pub.published) != 2 {
		t.Fatalf("expected 2 publishes, got %d", len(pub.published))
	}
}
