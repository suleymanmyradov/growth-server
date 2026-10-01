package db

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
)

// AIFeedback represents a row in the ai_feedback table.
type AIFeedback struct {
	ID        uuid.UUID          `db:"id" json:"id"`
	UserID    uuid.UUID          `db:"user_id" json:"user_id"`
	CheckInID uuid.UUID          `db:"check_in_id" json:"check_in_id"`
	HabitID   uuid.UUID          `db:"habit_id" json:"habit_id"`
	Content   string             `db:"content" json:"content"`
	Model     string             `db:"model" json:"model"`
	CreatedAt pgtype.Timestamptz `db:"created_at" json:"created_at"`
}

// CoachCheckIn represents a row in ai_coach_check_ins — the consumer-owned
// read model fed by check_in_created events (P2). It mirrors the check-ins
// fields the digest prompt needs, with habit_name denormalized from the event
// payload so no cross-service join is required.
type CoachCheckIn struct {
	CheckInID  uuid.UUID          `db:"check_in_id" json:"check_in_id"`
	UserID     uuid.UUID          `db:"user_id" json:"user_id"`
	HabitID    uuid.UUID          `db:"habit_id" json:"habit_id"`
	HabitName  string             `db:"habit_name" json:"habit_name"`
	Status     string             `db:"status" json:"status"`
	Mood       *string            `db:"mood" json:"mood"`
	Energy     *string            `db:"energy" json:"energy"`
	Blocker    *string            `db:"blocker" json:"blocker"`
	Note       *string            `db:"note" json:"note"`
	LocalDate  pgtype.Date        `db:"local_date" json:"local_date"`
	OccurredAt pgtype.Timestamptz `db:"occurred_at" json:"occurred_at"`
	CreatedAt  pgtype.Timestamptz `db:"created_at" json:"created_at"`
}

// CoachProfile represents a row in ai_coach_profiles — the consumer-owned
// read model fed by coaching_profile_changed events.
type CoachProfile struct {
	UserID              uuid.UUID `db:"user_id" json:"user_id"`
	AccountabilityStyle string    `db:"accountability_style" json:"accountability_style"`
}

// EventOutboxRow is a claimed ai_coach_event_outbox row.
type EventOutboxRow struct {
	EventID    uuid.UUID          `db:"event_id"`
	EventType  string             `db:"event_type"`
	Payload    []byte             `db:"payload"`
	OccurredAt pgtype.Timestamptz `db:"occurred_at"`
}

// InsertAIFeedbackParams holds parameters for inserting into ai_feedback.
// CheckInID and HabitID are nullable — daily digest rows have NULL for both
// since they cover multiple check-ins, not a single one.
type InsertAIFeedbackParams struct {
	ID        uuid.UUID  `db:"id" json:"id"`
	UserID    uuid.UUID  `db:"user_id" json:"user_id"`
	CheckInID *uuid.UUID `db:"check_in_id" json:"check_in_id"`
	HabitID   *uuid.UUID `db:"habit_id" json:"habit_id"`
	Content   string     `db:"content" json:"content"`
	Model     string     `db:"model" json:"model"`
}

// GetCheckInsForWeekParams holds parameters for the weekly check-in query.
type GetCheckInsForWeekParams struct {
	UserID      uuid.UUID `db:"user_id" json:"user_id"`
	OccurredAt  time.Time `db:"occurred_at" json:"occurred_at"`
	OccurredAt2 time.Time `db:"occurred_at_2" json:"occurred_at_2"`
}

// Queries is the set of database queries used by ai-coach-consumer.
type Queries struct {
	db DBTX
}

// New creates a new Queries instance backed by a pgx-compatible connection pool.
func New(db DBTX) *Queries {
	return &Queries{db: db}
}
