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
	claimed   []db.ClaimDueRemindersRow
	err       error
	markedSent []uuid.UUID
	released  int64
	releaseErr error
	markErr   error
}

func (f *fakeRepo) ClaimDueReminders(_ context.Context, _ int32) ([]db.ClaimDueRemindersRow, error) {
	return f.claimed, f.err
}

func (f *fakeRepo) MarkSent(_ context.Context, id uuid.UUID) (db.MarkReminderSentRow, error) {
	f.markedSent = append(f.markedSent, id)
	return db.MarkReminderSentRow{}, f.markErr
}

func (f *fakeRepo) ReleaseStaleClaims(_ context.Context, _ int32) (int64, error) {
	return f.released, f.releaseErr
}

type fakePub struct {
	published []events.Envelope
	err       error
	failOnIdx int // -1 = never fail; otherwise fail on this index
}

func (f *fakePub) Publish(_ context.Context, env events.Envelope) error {
	idx := len(f.published)
	f.published = append(f.published, env)
	if f.failOnIdx >= 0 && idx == f.failOnIdx {
		return f.err
	}
	if f.err != nil && f.failOnIdx < 0 {
		return f.err
	}
	return nil
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

func TestScheduler_Tick_ClaimPublishAck(t *testing.T) {
	uid := uuid.New()
	rid := uuid.New()
	repo := &fakeRepo{
		claimed: []db.ClaimDueRemindersRow{
			{ID: rid, UserID: uid, Type: "habit_reminder", ScheduledAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
		},
	}
	pub := &fakePub{}
	s := NewScheduler(repo, pub, fakeSchedClock{t: time.Now()})

	s.tick(context.Background())
	if len(pub.published) != 1 {
		t.Fatalf("expected 1 publish, got %d", len(pub.published))
	}
	if len(repo.markedSent) != 1 {
		t.Fatalf("expected 1 ack (mark sent), got %d", len(repo.markedSent))
	}
	if repo.markedSent[0] != rid {
		t.Errorf("expected ack for %s, got %s", rid, repo.markedSent[0])
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

func TestScheduler_Tick_PublishError_NoAck(t *testing.T) {
	uid := uuid.New()
	rid := uuid.New()
	repo := &fakeRepo{
		claimed: []db.ClaimDueRemindersRow{
			{ID: rid, UserID: uid, Type: "habit_reminder", ScheduledAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
		},
	}
	pub := &fakePub{err: context.DeadlineExceeded, failOnIdx: 0}
	s := NewScheduler(repo, pub, fakeSchedClock{t: time.Now()})

	s.tick(context.Background())
	// Publish was attempted, but since it failed, the reminder must NOT be
	// acked (marked sent). The lease will expire and it will be re-claimed.
	if len(repo.markedSent) != 0 {
		t.Fatalf("expected 0 acks on publish failure, got %d (reminder would be lost)", len(repo.markedSent))
	}
}

func TestScheduler_Tick_ClaimError(t *testing.T) {
	repo := &fakeRepo{err: context.Canceled}
	pub := &fakePub{}
	s := NewScheduler(repo, pub, fakeSchedClock{t: time.Now()})

	s.tick(context.Background())
	if len(pub.published) != 0 {
		t.Fatalf("expected 0 publishes on claim error, got %d", len(pub.published))
	}
}

func TestScheduler_MultipleReminders(t *testing.T) {
	uid := uuid.New()
	repo := &fakeRepo{
		claimed: []db.ClaimDueRemindersRow{
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
	if len(repo.markedSent) != 2 {
		t.Fatalf("expected 2 acks, got %d", len(repo.markedSent))
	}
}

func TestScheduler_Tick_PartialPublishFailure(t *testing.T) {
	uid := uuid.New()
	rid1 := uuid.New()
	rid2 := uuid.New()
	repo := &fakeRepo{
		claimed: []db.ClaimDueRemindersRow{
			{ID: rid1, UserID: uid, Type: "habit_reminder", ScheduledAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
			{ID: rid2, UserID: uid, Type: "weekly_review", ScheduledAt: pgtype.Timestamptz{Time: time.Now(), Valid: true}},
		},
	}
	pub := &fakePub{err: context.DeadlineExceeded, failOnIdx: 0}
	s := NewScheduler(repo, pub, fakeSchedClock{t: time.Now()})

	s.tick(context.Background())
	// First publish fails (no ack), second succeeds (ack).
	if len(repo.markedSent) != 1 {
		t.Fatalf("expected 1 ack (only the successful publish), got %d", len(repo.markedSent))
	}
	if repo.markedSent[0] != rid2 {
		t.Errorf("expected ack for %s (second reminder), got %s", rid2, repo.markedSent[0])
	}
}

func TestScheduler_Tick_ReleaseStaleClaims(t *testing.T) {
	repo := &fakeRepo{
		released: 3,
	}
	pub := &fakePub{}
	s := NewScheduler(repo, pub, fakeSchedClock{t: time.Now()})

	s.tick(context.Background())
	// Stale claims should be released before claiming new ones.
	if repo.released != 3 {
		t.Errorf("expected 3 stale claims released, got %d", repo.released)
	}
}
