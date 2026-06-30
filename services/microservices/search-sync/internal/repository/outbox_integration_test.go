//go:build integration

// Integration tests for the search-sync OutboxRepository memory getters and
// the migration 032 triggers. Requires the dev Postgres on localhost:5434.
//
// Run with:
//
//	go test -tags=integration -v ./services/microservices/search-sync/internal/repository/...
package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// TestMemoryGetters verifies the three new getters return correctly-shaped
// user_memory docs (id scheme, user_id, entity_type, content, metadata).
func TestMemoryGetters(t *testing.T) {
	pool := setupPool(t)
	defer pool.Close()
	ctx := context.Background()
	repo := NewOutboxRepository(pool)

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
	if ci["id"] != fmt.Sprintf("check_in_%s", checkInID) {
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
	if msg["id"] != fmt.Sprintf("conversation_message_%s", msgID) {
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
	if wr["id"] != fmt.Sprintf("weekly_review_%s", reviewID) {
		t.Errorf("review id = %v", wr["id"])
	}
	if wr["entity_type"] != "weekly_review" || wr["user_id"] != userID.String() {
		t.Errorf("review type/user = %v/%v", wr["entity_type"], wr["user_id"])
	}
	if wr["content"] != "Strong week" {
		t.Errorf("review content = %v", wr["content"])
	}
}

// TestMemoryTriggersEnqueue verifies migration 032 triggers enqueue the right
// outbox rows: note-bearing check_in upserts (empty note skipped), message
// upsert, weekly_review upsert, and cascade/per-row deletes.
func TestMemoryTriggersEnqueue(t *testing.T) {
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

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM search_outbox`); err != nil {
		t.Fatalf("clear outbox: %v", err)
	}

	// note-bearing check_in -> upsert
	if _, err := tx.Exec(ctx,
		`INSERT INTO check_ins (user_id, habit_id, local_date, status, note)
		 VALUES ($1, $2, '2002-01-01', 'completed', 'note here')`, userID, habitID); err != nil {
		t.Fatalf("insert check_in: %v", err)
	}
	// empty-note check_in -> NOT enqueued
	if _, err := tx.Exec(ctx,
		`INSERT INTO check_ins (user_id, habit_id, local_date, status, note)
		 VALUES ($1, $2, '2002-01-02', 'completed', '')`, userID, habitID); err != nil {
		t.Fatalf("insert empty check_in: %v", err)
	}
	// conversation + message -> upsert
	var convID uuid.UUID
	err = tx.QueryRow(ctx,
		`INSERT INTO conversations (user_id, title) VALUES ($1, 'trig') RETURNING id`,
		userID).Scan(&convID)
	if err != nil {
		t.Fatalf("insert conversation: %v", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO conversation_messages (conversation_id, role, content)
		 VALUES ($1, 'user', 'hello')`, convID); err != nil {
		t.Fatalf("insert message: %v", err)
	}
	// weekly_review -> upsert
	if _, err := tx.Exec(ctx,
		`INSERT INTO weekly_reviews (user_id, week_start, ai_summary)
		 VALUES ($1, '2002-01-06', 'summary')`, userID); err != nil {
		t.Fatalf("insert weekly_review: %v", err)
	}

	got := outboxCounts(t, tx)
	if got["check_in"] != 1 || got["conversation_message"] != 1 || got["weekly_review"] != 1 {
		t.Errorf("after inserts, outbox = %+v (want 1/1/1, empty-note skipped)", got)
	}

	// cascade delete conversation -> message delete
	if _, err := tx.Exec(ctx, `DELETE FROM conversations WHERE id = $1`, convID); err != nil {
		t.Fatalf("delete conversation: %v", err)
	}
	// delete check_ins -> check_in deletes (both rows; empty-note delete is a no-op index delete)
	if _, err := tx.Exec(ctx,
		`DELETE FROM check_ins WHERE user_id = $1 AND local_date IN ('2002-01-01','2002-01-02')`,
		userID); err != nil {
		t.Fatalf("delete check_ins: %v", err)
	}

	del := outboxOps(t, tx)
	if del["conversation_message:delete"] != 1 {
		t.Errorf("expected 1 conversation_message delete, got %+v", del)
	}
	if del["check_in:delete"] != 2 {
		t.Errorf("expected 2 check_in deletes, got %+v", del)
	}
}

// outboxCounts counts outbox rows per entity_type within the transaction.
func outboxCounts(t *testing.T, tx pgx.Tx) map[string]int {
	t.Helper()
	rows, err := tx.Query(context.Background(),
		`SELECT entity_type, count(*) FROM search_outbox GROUP BY entity_type`)
	if err != nil {
		t.Fatalf("query outbox counts: %v", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var et string
		var n int
		if err := rows.Scan(&et, &n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[et] = n
	}
	return out
}

// outboxOps counts outbox rows by "entity_type:operation" within the tx.
func outboxOps(t *testing.T, tx pgx.Tx) map[string]int {
	t.Helper()
	rows, err := tx.Query(context.Background(),
		`SELECT entity_type, operation, count(*) FROM search_outbox GROUP BY entity_type, operation`)
	if err != nil {
		t.Fatalf("query outbox ops: %v", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var et, op string
		var n int
		if err := rows.Scan(&et, &op, &n); err != nil {
			t.Fatalf("scan: %v", err)
		}
		out[et+":"+op] = n
	}
	return out
}

func cleanupMemoryRows(t *testing.T, pool *pgxpool.Pool, checkInID, msgID, reviewID, convID uuid.UUID) {
	t.Helper()
	ctx := context.Background()
	// Deleting the conversation cascades to its messages.
	_, _ = pool.Exec(ctx, `DELETE FROM conversations WHERE id = $1`, convID)
	_, _ = pool.Exec(ctx, `DELETE FROM check_ins WHERE id = $1`, checkInID)
	_, _ = pool.Exec(ctx, `DELETE FROM weekly_reviews WHERE id = $1`, reviewID)
	// Best-effort cleanup of any stragglers by id.
	_, _ = pool.Exec(ctx, `DELETE FROM conversation_messages WHERE id = $1`, msgID)
}
