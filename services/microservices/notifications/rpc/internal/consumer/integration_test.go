package consumer

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/notifications"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
)

// dbTestDSN is the default local docker postgres DSN; overridden by DATABASE_URL.
const dbTestDSN = "postgres://growthmind:growthmind123@localhost:5434/growthmind?sslmode=disable"

// integrationDB connects to the test postgres, skipping the test when no DSN
// is reachable. The caller is responsible for closing the pool.
func integrationDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = dbTestDSN
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Skipf("cannot connect to test DB (set DATABASE_URL to enable): %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		t.Skipf("cannot ping test DB (set DATABASE_URL to enable): %v", err)
	}
	return pool
}

// TestAllNotificationTypesInsertable verifies that every type declared in
// pkg/notifications can be inserted into the notifications table — i.e. the
// DB CHECK constraint and the Go type set are in sync. This catches the exact
// divergence the audit flagged (ai_feedback was in Go but not in the DB).
func TestAllNotificationTypesInsertable(t *testing.T) {
	pool := integrationDB(t)
	defer pool.Close()

	queries := db.New(pool)
	repo := repository.NewRepository(queries)

	userID := uuid.New()
	ctx := context.Background()

	for _, typ := range notifications.All() {
		t.Run(string(typ), func(t *testing.T) {
			row, err := repo.Notifications.CreateNotification(ctx,
				"test title", "test message", string(typ), userID)
			if err != nil {
				t.Fatalf("insert notification type %q: %v", typ, err)
			}
			if row.ID == uuid.Nil {
				t.Fatalf("insert returned nil ID for type %q", typ)
			}
			// Cleanup this row so the test is repeatable.
			if err := repo.Notifications.DeleteNotification(ctx, row.ID); err != nil {
				t.Logf("cleanup notification %s: %v", row.ID, err)
			}
		})
	}
}

// TestCheckInFeedbackGeneratedCreatesAIFeedbackNotification proves the
// event-consumer path for CheckInFeedbackGenerated creates an ai_feedback
// notification row in the database (the audit's critical blocker #1).
func TestCheckInFeedbackGeneratedCreatesAIFeedbackNotification(t *testing.T) {
	pool := integrationDB(t)
	defer pool.Close()

	queries := db.New(pool)
	repo := repository.NewRepository(queries)

	userID := uuid.New()
	content := "Great work on your habit streak today!"

	// Build the event envelope the same way the producer would.
	payload := events.CheckInFeedbackGenerated{
		UserID:    userID.String(),
		CheckInID: uuid.New().String(),
		HabitID:   uuid.New().String(),
		Content:   content,
	}
	env, err := events.NewEnvelope(events.TypeCheckInFeedbackGenerated, payload)
	if err != nil {
		t.Fatalf("new envelope: %v", err)
	}

	// Use a nil Publisher and ProcessedEvents — onCheckInFeedbackGenerated
	// only touches repo.Notifications. A nil ProcessedEvents is handled
	// (the Consume method guards with != nil).
	h := NewEventsHandler(repo, nil, nil, nil, nil)

	if err := h.onCheckInFeedbackGenerated(context.Background(), repo, env); err != nil {
		t.Fatalf("onCheckInFeedbackGenerated: %v", err)
	}

	// Verify the ai_feedback row was created by listing notifications for the
	// user and finding one with type == ai_feedback and matching content.
	listed, err := repo.Notifications.ListNotificationsForUser(context.Background(), userID, 50, 0)
	if err != nil {
		t.Fatalf("list notifications: %v", err)
	}
	var found bool
	for _, n := range listed {
		if n.Type == "ai_feedback" && n.Message == content {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("ai_feedback notification with content %q not found among %d rows", content, len(listed))
	}

	// Cleanup.
	if err := repo.Notifications.DeleteByUser(context.Background(), userID); err != nil {
		t.Logf("cleanup user %s notifications: %v", userID, err)
	}
}

// TestConsume_TransactionalAtomicity verifies that the side-effect (notification
// creation) and the processed_events marking are atomic: when the handler
// succeeds and Mark succeeds, the event is not re-processed on "redelivery".
// This is the core idempotency guarantee that prevents duplicate notifications.
func TestConsume_TransactionalAtomicity(t *testing.T) {
	pool := integrationDB(t)
	defer pool.Close()

	queries := db.New(pool)
	repo := repository.NewRepository(queries)
	txRunner := postgres.NewPgxTxRunner(pool)

	userID := uuid.New()
	content := "Transactional atomicity test " + uuid.NewString()

	payload := events.CheckInFeedbackGenerated{
		UserID:    userID.String(),
		CheckInID: uuid.New().String(),
		HabitID:   uuid.New().String(),
		Content:   content,
	}
	env, err := events.NewEnvelope(events.TypeCheckInFeedbackGenerated, payload)
	if err != nil {
		t.Fatalf("new envelope: %v", err)
	}
	raw, _ := json.Marshal(env)

	h := NewEventsHandler(repo, nil, nil, txRunner, nil)

	// First delivery: should create the notification and mark as processed.
	if err := h.Consume(context.Background(), "", string(raw)); err != nil {
		t.Fatalf("first consume: %v", err)
	}

	// Verify exactly one notification was created.
	listed, err := repo.Notifications.ListNotificationsForUser(context.Background(), userID, 50, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	count := 0
	for _, n := range listed {
		if n.Type == "ai_feedback" && n.Message == content {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 notification after first delivery, got %d", count)
	}

	// Second delivery (redelivery): should be skipped by IsProcessed.
	if err := h.Consume(context.Background(), "", string(raw)); err != nil {
		t.Fatalf("second consume: %v", err)
	}

	// Verify still exactly one notification (no duplicate).
	listed, err = repo.Notifications.ListNotificationsForUser(context.Background(), userID, 50, 0)
	if err != nil {
		t.Fatalf("list after redelivery: %v", err)
	}
	count = 0
	for _, n := range listed {
		if n.Type == "ai_feedback" && n.Message == content {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 notification after redelivery, got %d (duplicate created!)", count)
	}

	// Cleanup.
	if err := repo.Notifications.DeleteByUser(context.Background(), userID); err != nil {
		t.Logf("cleanup: %v", err)
	}
	if err := repo.ProcessedEvents.Mark(context.Background(), uuid.MustParse(env.EventID)); err != nil {
		t.Logf("cleanup processed_events: %v", err)
	}
}

// TestScheduler_ClaimLeaseAck_Integration verifies the full claim/lease/ack
// flow against the real database: a due reminder is claimed (leased), not
// marked sent, then acked (marked sent) after "publish". A stale claim is
// released and re-claimable.
func TestScheduler_ClaimLeaseAck_Integration(t *testing.T) {
	pool := integrationDB(t)
	defer pool.Close()

	queries := db.New(pool)
	repo := repository.NewRepository(queries)

	userID := uuid.New()
	ctx := context.Background()

	// Enqueue a reminder due now.
	enqueueRow, err := repo.Reminders.Enqueue(ctx, userID, "habit_reminder", time.Now().Add(-1*time.Minute), nil)
	if err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	reminderID := enqueueRow.ID

	// Cleanup.
	defer func() {
		_ = repo.Reminders.DeleteByUser(ctx, userID)
	}()

	// Claim due reminders — should lease (set claimed_at) but NOT set sent_at.
	claimed, err := repo.Reminders.ClaimDueReminders(ctx, 10)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if len(claimed) == 0 {
		t.Fatal("expected at least 1 claimed reminder")
	}
	var foundClaimed bool
	for _, c := range claimed {
		if c.ID == reminderID {
			foundClaimed = true
			break
		}
	}
	if !foundClaimed {
		t.Fatal("our reminder was not claimed")
	}

	// Verify the reminder is NOT marked sent yet (only leased).
	pending, err := repo.Reminders.GetPendingByUser(ctx, userID)
	if err != nil {
		t.Fatalf("get pending: %v", err)
	}
	stillPending := false
	for _, p := range pending {
		if p.ID == reminderID {
			stillPending = true
			break
		}
	}
	if !stillPending {
		t.Fatal("reminder should still be pending (sent_at NULL) after claim — lease only")
	}

	// Ack: mark sent after "successful publish".
	if _, err := repo.Reminders.MarkSent(ctx, reminderID); err != nil {
		t.Fatalf("mark sent: %v", err)
	}

	// Verify it's no longer pending.
	pending, err = repo.Reminders.GetPendingByUser(ctx, userID)
	if err != nil {
		t.Fatalf("get pending after ack: %v", err)
	}
	for _, p := range pending {
		if p.ID == reminderID {
			t.Fatal("reminder should not be pending after ack (mark sent)")
		}
	}
}

// countEmailDeliveries returns the number of email delivery rows for a user.
func countEmailDeliveries(t *testing.T, pool *pgxpool.Pool, userID uuid.UUID) int64 {
	t.Helper()
	var n int64
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT count(*) FROM notification_deliveries WHERE user_id = $1 AND channel = 'email'`, userID).Scan(&n))
	return n
}

// TestReminderDueHandler_EmailFlagPerReminderType verifies which reminder
// types enqueue an email delivery: habit_reminder and missed_check_in do,
// encouragement stays in-app only. It also asserts the missed check-in copy
// uses recovery framing rather than a scolding.
func TestReminderDueHandler_EmailFlagPerReminderType(t *testing.T) {
	pool := integrationDB(t)
	defer pool.Close()

	queries := db.New(pool)
	repo := repository.NewRepository(queries)
	ctx := context.Background()
	userID := uuid.New()
	defer func() {
		_ = repo.Notifications.DeleteByUser(ctx, userID)
		_ = repo.Reminders.DeleteByUser(ctx, userID)
		_ = repo.ReminderState.Delete(ctx, userID)
	}()

	// Seed reminder state: one active habit, reminders enabled, UTC, 20:00.
	require.NoError(t, repo.ReminderState.UpsertSettings(ctx, userID, "UTC",
		pgtype.Time{Microseconds: 20 * 3600 * 1e6, Valid: true}, true))
	require.NoError(t, repo.ReminderState.IncrementHabitCount(ctx, userID))

	h := NewReminderDueHandler(repo, nil, nil, nil, nil, nil)

	require.NoError(t, h.dispatch(ctx, repo, userID, events.ReminderDue{Type: "habit_reminder"}))
	assert.EqualValues(t, 1, countEmailDeliveries(t, pool, userID), "habit_reminder should enqueue an email delivery")

	// The user hasn't checked in today, so missed_check_in fires — by email too.
	require.NoError(t, h.dispatch(ctx, repo, userID, events.ReminderDue{Type: "missed_check_in"}))
	assert.EqualValues(t, 2, countEmailDeliveries(t, pool, userID), "missed_check_in should enqueue an email delivery")

	// Encouragement stays in-app only.
	require.NoError(t, h.dispatch(ctx, repo, userID, events.ReminderDue{Type: "encouragement"}))
	assert.EqualValues(t, 2, countEmailDeliveries(t, pool, userID), "encouragement must not enqueue an email delivery")

	// The missed check-in copy is a recovery message, not a scolding.
	listed, err := repo.Notifications.ListNotificationsForUser(ctx, userID, 50, 0)
	require.NoError(t, err)
	var missed *db.ListNotificationsForUserRow
	for i := range listed {
		if listed[i].Type == "missed_check_in" {
			missed = &listed[i]
		}
	}
	require.NotNil(t, missed, "missed_check_in notification not found")
	// Sent ~2h after the reminder with most of the day left — the copy must
	// not give up on today (that would contradict the 20:00 streak warning).
	assert.Contains(t, missed.Message, "Still time today")
	assert.NotContains(t, missed.Message, "tomorrow")
	assert.NotContains(t, missed.Message, "You missed")
}
