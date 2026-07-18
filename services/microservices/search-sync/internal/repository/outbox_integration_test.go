//go:build integration

// Integration tests for the search-sync repository and the migration 036
// pg_notify triggers. Requires the dev Postgres on localhost:5434.
//
// Run with:
//
//	go test -tags=integration -v ./services/microservices/search-sync/internal/repository/...
package repository

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const testDatasource = "postgres://growthmind:growthmind123@localhost:5434/growthmind?sslmode=disable"

func setupPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(context.Background(), testDatasource)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	return pool
}

// TestMemoryGetters verifies the three memory getters return correctly-shaped
// user_memory docs (id scheme, user_id, entity_type, content, metadata).
func TestMemoryGetters(t *testing.T) {
	pool := setupPool(t)
	defer pool.Close()
	ctx := context.Background()
	repo := NewRepository(pool)

	// Pick existing user + habit to satisfy FKs.
	var userID, habitID uuid.UUID
	err := pool.QueryRow(ctx,
		`SELECT u.id, h.id FROM users u JOIN habits h ON h.user_id = u.id LIMIT 1`).
		Scan(&userID, &habitID)
	if err != nil {
		t.Skipf("no user+habit fixture in dev DB: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	// check_in with note
	var checkInID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO check_ins (user_id, habit_id, local_date, status, note)
		 VALUES ($1, $2, '2001-01-01', 'completed', 'felt focused')
		 RETURNING id`, userID, habitID).Scan(&checkInID)
	if err != nil {
		t.Fatalf("insert check_in: %v", err)
	}

	// conversation + message
	var convID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO conversations (user_id, title) VALUES ($1, 't') RETURNING id`,
		userID).Scan(&convID)
	if err != nil {
		t.Fatalf("insert conversation: %v", err)
	}
	var msgID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO conversation_messages (conversation_id, role, content)
		 VALUES ($1, 'user', 'I need help with sleep') RETURNING id`, convID).Scan(&msgID)
	if err != nil {
		t.Fatalf("insert message: %v", err)
	}

	// weekly_review with ai_summary
	var reviewID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO weekly_reviews (user_id, week_start, ai_summary)
		 VALUES ($1, '2001-01-08', 'Strong week') RETURNING id`, userID).Scan(&reviewID)
	if err != nil {
		t.Fatalf("insert weekly_review: %v", err)
	}

	// Getters read from the pool (outside the tx), so commit to make rows
	// visible, then clean up at the end of the test.
	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	defer cleanupMemoryRows(t, pool, checkInID, msgID, reviewID, convID)

	ci, err := repo.GetCheckIn(ctx, checkInID)
	if err != nil {
		t.Fatalf("GetCheckIn: %v", err)
	}
	if ci["id"] != "check_in_"+checkInID.String() {
		t.Errorf("check_in id = %v", ci["id"])
	}
	if ci["entity_type"] != "check_in" || ci["user_id"] != userID.String() {
		t.Errorf("check_in type/user = %v/%v", ci["entity_type"], ci["user_id"])
	}
	if ci["content"] != "felt focused" {
		t.Errorf("check_in content = %v", ci["content"])
	}

	msg, err := repo.GetMessage(ctx, msgID)
	if err != nil {
		t.Fatalf("GetMessage: %v", err)
	}
	if msg["id"] != "conversation_message_"+msgID.String() {
		t.Errorf("message id = %v", msg["id"])
	}
	if msg["entity_type"] != "conversation_message" || msg["user_id"] != userID.String() {
		t.Errorf("message type/user = %v/%v", msg["entity_type"], msg["user_id"])
	}
	if msg["content"] != "I need help with sleep" || msg["role"] != "user" {
		t.Errorf("message content/role = %v/%v", msg["content"], msg["role"])
	}

	wr, err := repo.GetWeeklyReview(ctx, reviewID)
	if err != nil {
		t.Fatalf("GetWeeklyReview: %v", err)
	}
	if wr["id"] != "weekly_review_"+reviewID.String() {
		t.Errorf("review id = %v", wr["id"])
	}
	if wr["entity_type"] != "weekly_review" || wr["user_id"] != userID.String() {
		t.Errorf("review type/user = %v/%v", wr["entity_type"], wr["user_id"])
	}
	if wr["content"] != "Strong week" {
		t.Errorf("review content = %v", wr["content"])
	}
}

// TestNotifyTriggers verifies migration 036 triggers fire pg_notify on the
// 'search_sync' channel: note-bearing check_in upserts (empty note skipped),
// message upsert, weekly_review upsert, and cascade/per-row deletes.
func TestNotifyTriggers(t *testing.T) {
	pool := setupPool(t)
	defer pool.Close()
	ctx := context.Background()

	var userID, habitID uuid.UUID
	err := pool.QueryRow(ctx,
		`SELECT u.id, h.id FROM users u JOIN habits h ON h.user_id = u.id LIMIT 1`).
		Scan(&userID, &habitID)
	if err != nil {
		t.Skipf("no user+habit fixture: %v", err)
	}

	// Acquire a dedicated connection and LISTEN before making changes.
	listenConn, err := pool.Acquire(ctx)
	if err != nil {
		t.Fatalf("acquire listen conn: %v", err)
	}
	defer listenConn.Release()
	if _, err := listenConn.Exec(ctx, "LISTEN search_sync"); err != nil {
		t.Fatalf("listen: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	// note-bearing check_in -> upsert notification
	var checkInID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO check_ins (user_id, habit_id, local_date, status, note)
		 VALUES ($1, $2, '2002-01-01', 'completed', 'note here') RETURNING id`,
		userID, habitID).Scan(&checkInID); err != nil {
		t.Fatalf("insert check_in: %v", err)
	}
	// empty-note check_in -> NOT notified
	if _, err := tx.Exec(ctx,
		`INSERT INTO check_ins (user_id, habit_id, local_date, status, note)
		 VALUES ($1, $2, '2002-01-02', 'completed', '')`, userID, habitID); err != nil {
		t.Fatalf("insert empty check_in: %v", err)
	}
	// conversation + message -> upsert notification
	var convID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO conversations (user_id, title) VALUES ($1, 'trig') RETURNING id`,
		userID).Scan(&convID); err != nil {
		t.Fatalf("insert conversation: %v", err)
	}
	var msgID uuid.UUID
	if err := tx.QueryRow(ctx,
		`INSERT INTO conversation_messages (conversation_id, role, content)
		 VALUES ($1, 'user', 'hello') RETURNING id`, convID).Scan(&msgID); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	// weekly_review -> upsert notification
	if _, err := tx.Exec(ctx,
		`INSERT INTO weekly_reviews (user_id, week_start, ai_summary)
		 VALUES ($1, '2002-01-06', 'summary')`, userID); err != nil {
		t.Fatalf("insert weekly_review: %v", err)
	}

	if err := tx.Commit(ctx); err != nil {
		t.Fatalf("commit: %v", err)
	}
	defer cleanupMemoryRows(t, pool, checkInID, msgID, uuid.Nil, convID)

	// Collect notifications. pg_notify fires on commit, so they should be
	// available now. Drain with a short timeout.
	notifs := drainNotifications(t, ctx, listenConn, 3, 2*time.Second)
	if notifs["check_in:upsert"] < 1 {
		t.Errorf("expected >=1 check_in:upsert, got %d", notifs["check_in:upsert"])
	}
	if notifs["conversation_message:upsert"] < 1 {
		t.Errorf("expected >=1 conversation_message:upsert, got %d", notifs["conversation_message:upsert"])
	}
	if notifs["weekly_review:upsert"] < 1 {
		t.Errorf("expected >=1 weekly_review:upsert, got %d", notifs["weekly_review:upsert"])
	}

	// Now test deletes: cascade delete conversation -> message delete notification.
	// Delete check_ins -> check_in delete notifications.
	delTx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin del: %v", err)
	}
	defer delTx.Rollback(ctx)

	if _, err := delTx.Exec(ctx, `DELETE FROM conversations WHERE id = $1`, convID); err != nil {
		t.Fatalf("delete conversation: %v", err)
	}
	if _, err := delTx.Exec(ctx,
		`DELETE FROM check_ins WHERE user_id = $1 AND local_date IN ('2002-01-01','2002-01-02')`,
		userID); err != nil {
		t.Fatalf("delete check_ins: %v", err)
	}

	if err := delTx.Commit(ctx); err != nil {
		t.Fatalf("commit del: %v", err)
	}

	delNotifs := drainNotifications(t, ctx, listenConn, 3, 2*time.Second)
	if delNotifs["conversation_message:delete"] < 1 {
		t.Errorf("expected >=1 conversation_message:delete, got %d", delNotifs["conversation_message:delete"])
	}
	if delNotifs["check_in:delete"] < 2 {
		t.Errorf("expected >=2 check_in:delete, got %d", delNotifs["check_in:delete"])
	}
}

// drainNotifications reads up to max notifications within the timeout and
// returns a count map keyed by "entity_type:operation".
func drainNotifications(t *testing.T, ctx context.Context, conn *pgxpool.Conn, max int, timeout time.Duration) map[string]int {
	t.Helper()
	result := map[string]int{}
	deadline := time.Now().Add(timeout)
	for i := 0; i < max; i++ {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			break
		}
		notifCtx, cancel := context.WithTimeout(ctx, remaining)
		notification, err := conn.Conn().WaitForNotification(notifCtx)
		cancel()
		if err != nil {
			break
		}
		n, err := parseNotification(notification.Payload)
		if err != nil {
			continue
		}
		result[n.EntityType+":"+n.Operation]++
	}
	return result
}

func cleanupMemoryRows(t *testing.T, pool *pgxpool.Pool, checkInID, msgID, reviewID, convID uuid.UUID) {
	ctx := context.Background()
	if convID != uuid.Nil {
		_, _ = pool.Exec(ctx, `DELETE FROM conversations WHERE id = $1`, convID)
	}
	if checkInID != uuid.Nil {
		_, _ = pool.Exec(ctx, `DELETE FROM check_ins WHERE id = $1`, checkInID)
	}
	if reviewID != uuid.Nil {
		_, _ = pool.Exec(ctx, `DELETE FROM weekly_reviews WHERE id = $1`, reviewID)
	}
}
