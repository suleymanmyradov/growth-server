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

// CheckIn represents a row in the check_ins table (read-only for ai-coach).
type CheckIn struct {
	ID        uuid.UUID          `db:"id" json:"id"`
	UserID    uuid.UUID          `db:"user_id" json:"user_id"`
	HabitID   uuid.UUID          `db:"habit_id" json:"habit_id"`
	Status    string             `db:"status" json:"status"`
	Mood      *string            `db:"mood" json:"mood"`
	Energy    *string            `db:"energy" json:"energy"`
	Blocker   *string            `db:"blocker" json:"blocker"`
	Note      *string            `db:"note" json:"note"`
	CreatedAt pgtype.Timestamptz `db:"created_at" json:"created_at"`
}

// UserSetting represents the subset of user_settings needed by ai-coach.
type UserSetting struct {
	UserID              uuid.UUID `db:"user_id" json:"user_id"`
	AccountabilityStyle string    `db:"accountability_style" json:"accountability_style"`
}

// InsertAIFeedbackParams holds parameters for inserting into ai_feedback.
type InsertAIFeedbackParams struct {
	ID        uuid.UUID `db:"id" json:"id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	CheckInID uuid.UUID `db:"check_in_id" json:"check_in_id"`
	HabitID   uuid.UUID `db:"habit_id" json:"habit_id"`
	Content   string    `db:"content" json:"content"`
	Model     string    `db:"model" json:"model"`
}

// GetCheckInsForWeekParams holds parameters for the weekly check-in query.
type GetCheckInsForWeekParams struct {
	UserID     uuid.UUID `db:"user_id" json:"user_id"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	CreatedAt2 time.Time `db:"created_at_2" json:"created_at_2"`
}

// Queries is the set of database queries used by ai-coach.
type Queries struct {
	db DBTX
}

// New creates a new Queries instance backed by a pgx-compatible connection pool.
func New(db DBTX) *Queries {
	return &Queries{db: db}
}
