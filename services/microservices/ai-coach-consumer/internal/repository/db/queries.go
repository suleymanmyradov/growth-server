package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// All queries below touch ONLY tables owned by ai-coach-consumer
// (ai_feedback, ai_coach_check_ins, ai_coach_profiles,
// ai_coach_processed_events, ai_coach_event_outbox). Client-owned tables are
// mirrored via events into the read models — never read directly.

const insertAIFeedback = `INSERT INTO ai_feedback (id, user_id, check_in_id, habit_id, content, model)
VALUES ($1, $2, $3, $4, $5, $6)`

func (q *Queries) InsertAIFeedback(ctx context.Context, arg InsertAIFeedbackParams) error {
	_, err := q.db.Exec(ctx, insertAIFeedback,
		arg.ID, arg.UserID, arg.CheckInID, arg.HabitID, arg.Content, arg.Model,
	)
	if err != nil {
		return fmt.Errorf("insert ai_feedback: %w", err)
	}
	return nil
}

const deleteAIFeedbackByUser = `DELETE FROM ai_feedback WHERE user_id = $1`

func (q *Queries) DeleteAIFeedbackByUser(ctx context.Context, userID uuid.UUID) error {
	_, err := q.db.Exec(ctx, deleteAIFeedbackByUser, userID)
	if err != nil {
		return fmt.Errorf("delete ai_feedback by user: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ai_coach_check_ins — read model fed by check_in_created events
// ---------------------------------------------------------------------------

const upsertCoachCheckIn = `INSERT INTO ai_coach_check_ins
    (check_in_id, user_id, habit_id, habit_name, status, mood, energy, blocker, note, local_date, occurred_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NULLIF($10, '')::date, $11)
ON CONFLICT (check_in_id) DO UPDATE SET
    status     = EXCLUDED.status,
    mood       = EXCLUDED.mood,
    energy     = EXCLUDED.energy,
    blocker    = EXCLUDED.blocker,
    note       = EXCLUDED.note,
    habit_name = EXCLUDED.habit_name,
    local_date = EXCLUDED.local_date,
    occurred_at = EXCLUDED.occurred_at`

// UpsertCoachCheckInParams carries the check_in_created payload fields into
// the read model. LocalDate is "YYYY-MM-DD"; empty becomes NULL.
type UpsertCoachCheckInParams struct {
	CheckInID  uuid.UUID
	UserID     uuid.UUID
	HabitID    uuid.UUID
	HabitName  string
	Status     string
	Mood       *string
	Energy     *string
	Blocker    *string
	Note       *string
	LocalDate  string
	OccurredAt time.Time
}

// UpsertCoachCheckIn mirrors one check_in_created event into the read model.
// Upserting on check_in_id keeps the row truthful when a user re-checks-in
// (missed → completed) on the same day.
func (q *Queries) UpsertCoachCheckIn(ctx context.Context, arg UpsertCoachCheckInParams) error {
	_, err := q.db.Exec(ctx, upsertCoachCheckIn,
		arg.CheckInID, arg.UserID, arg.HabitID, arg.HabitName, arg.Status,
		arg.Mood, arg.Energy, arg.Blocker, arg.Note, arg.LocalDate, arg.OccurredAt,
	)
	if err != nil {
		return fmt.Errorf("upsert ai_coach_check_ins: %w", err)
	}
	return nil
}

const getCheckInsForWeek = `SELECT check_in_id, user_id, habit_id, habit_name, status, mood, energy, blocker, note, local_date, occurred_at, created_at
FROM ai_coach_check_ins
WHERE user_id = $1
  AND occurred_at >= $2
  AND occurred_at <= $3
ORDER BY occurred_at DESC`

func (q *Queries) GetCheckInsForWeek(ctx context.Context, arg GetCheckInsForWeekParams) ([]CoachCheckIn, error) {
	rows, err := q.db.Query(ctx, getCheckInsForWeek, arg.UserID, arg.OccurredAt, arg.OccurredAt2)
	if err != nil {
		return nil, fmt.Errorf("get check-ins for week: %w", err)
	}
	defer rows.Close()

	var items []CoachCheckIn
	for rows.Next() {
		var c CoachCheckIn
		if err := rows.Scan(&c.CheckInID, &c.UserID, &c.HabitID, &c.HabitName, &c.Status,
			&c.Mood, &c.Energy, &c.Blocker, &c.Note, &c.LocalDate, &c.OccurredAt, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan check-in: %w", err)
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

const getCheckInsForDate = `SELECT check_in_id, user_id, habit_id, habit_name, status, mood, energy, blocker, note, local_date, occurred_at, created_at
FROM ai_coach_check_ins
WHERE user_id = $1
  AND local_date = $2::date
ORDER BY occurred_at ASC`

// GetCheckInsForDate returns all check-ins for a user on a specific local
// date. Used by the daily coach digest to build a single prompt covering all
// of the day's check-ins.
func (q *Queries) GetCheckInsForDate(ctx context.Context, userID uuid.UUID, localDate string) ([]CoachCheckIn, error) {
	rows, err := q.db.Query(ctx, getCheckInsForDate, userID, localDate)
	if err != nil {
		return nil, fmt.Errorf("get check-ins for date: %w", err)
	}
	defer rows.Close()

	var items []CoachCheckIn
	for rows.Next() {
		var c CoachCheckIn
		if err := rows.Scan(&c.CheckInID, &c.UserID, &c.HabitID, &c.HabitName,
			&c.Status, &c.Mood, &c.Energy, &c.Blocker, &c.Note,
			&c.LocalDate, &c.OccurredAt, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan check-in: %w", err)
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

const deleteCoachCheckInsByUser = `DELETE FROM ai_coach_check_ins WHERE user_id = $1`

func (q *Queries) DeleteCoachCheckInsByUser(ctx context.Context, userID uuid.UUID) error {
	_, err := q.db.Exec(ctx, deleteCoachCheckInsByUser, userID)
	if err != nil {
		return fmt.Errorf("delete ai_coach_check_ins by user: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ai_coach_profiles — read model fed by coaching_profile_changed events
// ---------------------------------------------------------------------------

const upsertCoachProfile = `INSERT INTO ai_coach_profiles (user_id, accountability_style)
VALUES ($1, $2)
ON CONFLICT (user_id) DO UPDATE SET
    accountability_style = EXCLUDED.accountability_style,
    updated_at = now()`

func (q *Queries) UpsertCoachProfile(ctx context.Context, userID uuid.UUID, style string) error {
	_, err := q.db.Exec(ctx, upsertCoachProfile, userID, style)
	if err != nil {
		return fmt.Errorf("upsert ai_coach_profiles: %w", err)
	}
	return nil
}

const getAccountabilityStyle = `SELECT accountability_style
FROM ai_coach_profiles
WHERE user_id = $1`

func (q *Queries) GetAccountabilityStyle(ctx context.Context, userID uuid.UUID) (string, error) {
	var style string
	err := q.db.QueryRow(ctx, getAccountabilityStyle, userID).Scan(&style)
	if err != nil {
		return "", fmt.Errorf("get accountability style: %w", err)
	}
	return style, nil
}

const deleteCoachProfileByUser = `DELETE FROM ai_coach_profiles WHERE user_id = $1`

func (q *Queries) DeleteCoachProfileByUser(ctx context.Context, userID uuid.UUID) error {
	_, err := q.db.Exec(ctx, deleteCoachProfileByUser, userID)
	if err != nil {
		return fmt.Errorf("delete ai_coach_profiles by user: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ai_coach_processed_events — consumer dedup (owned here; the notifications
// service owns the generic processed_events table)
// ---------------------------------------------------------------------------

const markProcessed = `INSERT INTO ai_coach_processed_events (consumer, event_id)
VALUES ('ai_coach', $1)
ON CONFLICT DO NOTHING`

func (q *Queries) MarkProcessed(ctx context.Context, eventID uuid.UUID) error {
	_, err := q.db.Exec(ctx, markProcessed, eventID.String())
	if err != nil {
		return fmt.Errorf("mark processed: %w", err)
	}
	return nil
}

const isProcessed = `SELECT EXISTS(SELECT 1 FROM ai_coach_processed_events WHERE consumer = 'ai_coach' AND event_id = $1)`

func (q *Queries) IsProcessed(ctx context.Context, eventID uuid.UUID) (bool, error) {
	var exists bool
	err := q.db.QueryRow(ctx, isProcessed, eventID.String()).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("is processed: %w", err)
	}
	return exists, nil
}

// ---------------------------------------------------------------------------
// ai_coach_event_outbox — transactional outbox for published events (P1).
// Rows are written inside the digest transaction; the relay republishes with
// the stored event ID and deletes the row on success.
// ---------------------------------------------------------------------------

const enqueueEvent = `INSERT INTO ai_coach_event_outbox (event_id, event_type, payload, occurred_at)
VALUES ($1, $2, $3, $4)`

func (q *Queries) EnqueueEvent(ctx context.Context, eventID uuid.UUID, eventType string, payload []byte, occurredAt pgtype.Timestamptz) error {
	_, err := q.db.Exec(ctx, enqueueEvent, eventID, eventType, payload, occurredAt)
	if err != nil {
		return fmt.Errorf("enqueue event %s: %w", eventID, err)
	}
	return nil
}

const claimEvent = `UPDATE ai_coach_event_outbox
SET next_attempt_at = now() + interval '60 seconds',
    attempts = attempts + 1
WHERE event_id = (
    SELECT event_id FROM ai_coach_event_outbox
    WHERE next_attempt_at <= now()
    ORDER BY created_at
    LIMIT 1
    FOR UPDATE SKIP LOCKED
)
RETURNING event_id, event_type, payload, occurred_at`

func (q *Queries) ClaimEvent(ctx context.Context) (EventOutboxRow, error) {
	var row EventOutboxRow
	err := q.db.QueryRow(ctx, claimEvent).Scan(&row.EventID, &row.EventType, &row.Payload, &row.OccurredAt)
	if err != nil {
		return EventOutboxRow{}, err
	}
	return row, nil
}

const completeEvent = `DELETE FROM ai_coach_event_outbox WHERE event_id = $1`

func (q *Queries) CompleteEvent(ctx context.Context, eventID uuid.UUID) error {
	_, err := q.db.Exec(ctx, completeEvent, eventID)
	if err != nil {
		return fmt.Errorf("complete event %s: %w", eventID, err)
	}
	return nil
}

// DBTX is the common interface for pgx database operations.
type DBTX interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}

// NewWithTx creates a Queries using the given pgx transaction.
func NewWithTx(db DBTX) *Queries {
	return &Queries{db: db}
}
