package delivery

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/suleymanmyradov/growth-server/pkg/email"
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

// fakeEmailSender captures sends for assertions.
type fakeEmailSender struct {
	sent []email.Email
	err  error
}

func (f *fakeEmailSender) Send(_ context.Context, m email.Email) error {
	f.sent = append(f.sent, m)
	return f.err
}

// newEmailDelivery inserts a notification plus its pending email delivery row.
func newEmailDelivery(t *testing.T, ctx context.Context, repo *repository.Repository, userID uuid.UUID, notifType string) db.NotificationDelivery {
	t.Helper()
	n, err := repo.Notifications.Create(ctx, db.CreateNotificationParams{
		Title:    "title",
		Message:  "message",
		Type:     notifType,
		UserID:   userID,
		Metadata: []byte("{}"),
	})
	require.NoError(t, err)
	d, err := repo.Deliveries.Create(ctx, n.ID, userID, "email")
	require.NoError(t, err)
	return d
}

// deliveryState reads back the terminal status and suppression code.
func deliveryState(t *testing.T, ctx context.Context, pool *pgxpool.Pool, id uuid.UUID) (status, code string) {
	t.Helper()
	var c *string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT status, last_error_code FROM notification_deliveries WHERE id = $1`, id).
		Scan(&status, &c))
	if c != nil {
		code = *c
	}
	return status, code
}

// cleanupUser removes every row the tests create for a user. Deliveries
// cascade from notifications; the rest are per-user keyed tables.
func cleanupUser(t *testing.T, ctx context.Context, repo *repository.Repository, userID uuid.UUID) {
	t.Helper()
	if err := repo.Notifications.DeleteByUser(ctx, userID); err != nil {
		t.Logf("cleanup notifications: %v", err)
	}
	if err := repo.Recipients.Delete(ctx, userID); err != nil {
		t.Logf("cleanup recipient: %v", err)
	}
	if err := repo.ReminderState.Delete(ctx, userID); err != nil {
		t.Logf("cleanup reminder state: %v", err)
	}
	if err := repo.Preferences.DeleteByUser(ctx, userID); err != nil {
		t.Logf("cleanup preferences: %v", err)
	}
}

// TestWorker_DeliverEmail_SendsAndMarksSent covers the happy path: a verified
// recipient under the cap gets the email, with the unsubscribe headers and the
// delivery-row idempotency key wired through.
func TestWorker_DeliverEmail_SendsAndMarksSent(t *testing.T) {
	pool := integrationDB(t)
	defer pool.Close()
	repo := repository.NewRepository(db.New(pool))
	ctx := context.Background()
	userID := uuid.New()
	defer cleanupUser(t, ctx, repo, userID)

	_, err := repo.Recipients.Upsert(ctx, userID, "ada@example.com", "Ada", true)
	require.NoError(t, err)

	fake := &fakeEmailSender{}
	w := NewWorker(repo, nil, fake, "https://app.example.com", "https://api.example.com", "secret", true)

	d := newEmailDelivery(t, ctx, repo, userID, "habit_reminder")
	require.NoError(t, w.deliver(ctx, d))

	require.Len(t, fake.sent, 1)
	sent := fake.sent[0]
	assert.Equal(t, []string{"ada@example.com"}, sent.To)
	assert.Equal(t, d.ID.String(), sent.IdempotencyKey)
	assert.Contains(t, sent.Headers["List-Unsubscribe"], "email-unsubscribe")

	status, _ := deliveryState(t, ctx, pool, d.ID)
	assert.Equal(t, "sent", status)
}

// TestWorker_DeliverEmail_DailyCap verifies the 2-emails-per-local-day cap:
// the day's third email (the streak warning, last in the reminder → missed
// check-in → streak warning schedule) is suppressed while the earlier sends
// went through.
func TestWorker_DeliverEmail_DailyCap(t *testing.T) {
	pool := integrationDB(t)
	defer pool.Close()
	repo := repository.NewRepository(db.New(pool))
	ctx := context.Background()
	userID := uuid.New()
	defer cleanupUser(t, ctx, repo, userID)

	_, err := repo.Recipients.Upsert(ctx, userID, "ada@example.com", "Ada", true)
	require.NoError(t, err)
	// streak_warning is pref-guarded before the channel switch, and a missing
	// preferences row defaults StreakWarnings to false — seed it enabled.
	_, err = repo.Preferences.Upsert(ctx, db.UpsertNotificationPreferencesParams{
		UserID:             userID,
		EmailNotifications: true,
		PushNotifications:  true,
		HabitReminders:     true,
		GoalReminders:      true,
		StreakWarnings:     true,
		SundayReview:       true,
	})
	require.NoError(t, err)

	fake := &fakeEmailSender{}
	w := NewWorker(repo, nil, fake, "https://app.example.com", "https://api.example.com", "secret", true)

	// First email of the day: the check-in reminder actually sends.
	d1 := newEmailDelivery(t, ctx, repo, userID, "habit_reminder")
	require.NoError(t, w.deliver(ctx, d1))

	// Second email of the day (the missed check-in +2h later) — mark sent
	// directly so the test is independent of the hour it runs at.
	d2 := newEmailDelivery(t, ctx, repo, userID, "missed_check_in")
	require.NoError(t, repo.Deliveries.MarkSent(ctx, d2.ID, nil))

	// Third (the 20:00 streak warning) is over the cap.
	d3 := newEmailDelivery(t, ctx, repo, userID, "streak_warning")
	require.NoError(t, w.deliver(ctx, d3))

	require.Len(t, fake.sent, 1)
	status, code := deliveryState(t, ctx, pool, d3.ID)
	assert.Equal(t, "suppressed", status)
	assert.Equal(t, "daily_cap", code)
}

// TestWorker_DeliverEmail_SkipsWhenCheckedIn verifies the just-before-send
// re-check: a reminder email for a user who already checked in today is
// suppressed, because a retried email can arrive hours late.
func TestWorker_DeliverEmail_SkipsWhenCheckedIn(t *testing.T) {
	pool := integrationDB(t)
	defer pool.Close()
	repo := repository.NewRepository(db.New(pool))
	ctx := context.Background()
	userID := uuid.New()
	defer cleanupUser(t, ctx, repo, userID)

	_, err := repo.Recipients.Upsert(ctx, userID, "ada@example.com", "Ada", true)
	require.NoError(t, err)
	// A check-in today (user timezone falls back to UTC — no reminder_state
	// timezone is seeded here).
	require.NoError(t, repo.ReminderState.BumpCheckInCountToday(ctx, userID, time.Now().UTC()))

	fake := &fakeEmailSender{}
	w := NewWorker(repo, nil, fake, "https://app.example.com", "https://api.example.com", "secret", true)

	for _, notifType := range []string{"habit_reminder", "missed_check_in"} {
		d := newEmailDelivery(t, ctx, repo, userID, notifType)
		require.NoError(t, w.deliver(ctx, d))
		status, code := deliveryState(t, ctx, pool, d.ID)
		assert.Equal(t, "suppressed", status, notifType)
		assert.Equal(t, "already_checked_in", code, notifType)
	}
	assert.Empty(t, fake.sent)
}

// TestWorker_DeliverEmail_ReminderEmailsKillSwitch verifies the
// Email.ReminderEmailsEnabled flag suppresses reminder emails while other
// emails still send.
func TestWorker_DeliverEmail_ReminderEmailsKillSwitch(t *testing.T) {
	pool := integrationDB(t)
	defer pool.Close()
	repo := repository.NewRepository(db.New(pool))
	ctx := context.Background()
	userID := uuid.New()
	defer cleanupUser(t, ctx, repo, userID)

	_, err := repo.Recipients.Upsert(ctx, userID, "ada@example.com", "Ada", true)
	require.NoError(t, err)

	fake := &fakeEmailSender{}
	w := NewWorker(repo, nil, fake, "https://app.example.com", "https://api.example.com", "secret", false)

	for _, notifType := range []string{"habit_reminder", "missed_check_in", "goal_deadline"} {
		d := newEmailDelivery(t, ctx, repo, userID, notifType)
		require.NoError(t, w.deliver(ctx, d))
		status, code := deliveryState(t, ctx, pool, d.ID)
		assert.Equal(t, "suppressed", status, notifType)
		assert.Equal(t, "reminder_emails_disabled", code, notifType)
	}

	// Non-reminder emails are unaffected by the flag.
	d := newEmailDelivery(t, ctx, repo, userID, "ai_feedback")
	require.NoError(t, w.deliver(ctx, d))
	status, _ := deliveryState(t, ctx, pool, d.ID)
	assert.Equal(t, "sent", status)
	require.Len(t, fake.sent, 1)
}

// TestWorker_DeliverEmail_UserDisabledEmail verifies that turning email off in
// user settings suppresses every email delivery.
func TestWorker_DeliverEmail_UserDisabledEmail(t *testing.T) {
	pool := integrationDB(t)
	defer pool.Close()
	repo := repository.NewRepository(db.New(pool))
	ctx := context.Background()
	userID := uuid.New()
	defer cleanupUser(t, ctx, repo, userID)

	_, err := repo.Recipients.Upsert(ctx, userID, "ada@example.com", "Ada", true)
	require.NoError(t, err)
	_, err = repo.Preferences.Upsert(ctx, db.UpsertNotificationPreferencesParams{
		UserID:             userID,
		EmailNotifications: false,
		PushNotifications:  true,
		HabitReminders:     true,
		GoalReminders:      true,
		StreakWarnings:     true,
		SundayReview:       true,
	})
	require.NoError(t, err)

	fake := &fakeEmailSender{}
	w := NewWorker(repo, nil, fake, "https://app.example.com", "https://api.example.com", "secret", true)

	for _, notifType := range []string{"habit_reminder", "missed_check_in", "ai_feedback"} {
		d := newEmailDelivery(t, ctx, repo, userID, notifType)
		require.NoError(t, w.deliver(ctx, d))
		status, code := deliveryState(t, ctx, pool, d.ID)
		assert.Equal(t, "suppressed", status, notifType)
		assert.Equal(t, "email_disabled", code, notifType)
	}
	assert.Empty(t, fake.sent)
}
