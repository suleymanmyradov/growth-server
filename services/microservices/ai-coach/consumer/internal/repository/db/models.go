package db

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
)

// AIFeedback represents a row in the ai_feedback table.
type AIFeedback struct {
	ID        uuid.UUID `db:"id" json:"id"`
	UserID    uuid.UUID `db:"user_id" json:"user_id"`
	CheckInID uuid.UUID `db:"check_in_id" json:"check_in_id"`
	HabitID   uuid.UUID `db:"habit_id" json:"habit_id"`
	Content   string    `db:"content" json:"content"`
	Model     string    `db:"model" json:"model"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// CheckIn represents a row in the check_ins table (read-only for ai-coach).
type CheckIn struct {
	ID        uuid.UUID      `db:"id" json:"id"`
	UserID    uuid.UUID      `db:"user_id" json:"user_id"`
	HabitID   uuid.UUID      `db:"habit_id" json:"habit_id"`
	Status    string         `db:"status" json:"status"`
	Mood      sql.NullString `db:"mood" json:"mood"`
	Energy    sql.NullString `db:"energy" json:"energy"`
	Blocker   sql.NullString `db:"blocker" json:"blocker"`
	Note      sql.NullString `db:"note" json:"note"`
	CreatedAt time.Time      `db:"created_at" json:"created_at"`
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
	db *sql.DB
}

// New creates a new Queries instance.
func New(db *sql.DB) *Queries {
	return &Queries{db: db}
}
