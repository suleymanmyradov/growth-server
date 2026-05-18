package db

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
)

const insertAIFeedback = `INSERT INTO ai_feedback (id, user_id, check_in_id, habit_id, content, model)
VALUES ($1, $2, $3, $4, $5, $6)`

func (q *Queries) InsertAIFeedback(ctx context.Context, arg InsertAIFeedbackParams) error {
	_, err := q.db.ExecContext(ctx, insertAIFeedback,
		arg.ID, arg.UserID, arg.CheckInID, arg.HabitID, arg.Content, arg.Model,
	)
	if err != nil {
		return fmt.Errorf("insert ai_feedback: %w", err)
	}
	return nil
}

const getCheckInsForWeek = `SELECT id, user_id, habit_id, status, mood, energy, blocker, note, created_at
FROM check_ins
WHERE user_id = $1
  AND created_at >= $2
  AND created_at <= $3
ORDER BY created_at DESC`

func (q *Queries) GetCheckInsForWeek(ctx context.Context, arg GetCheckInsForWeekParams) ([]CheckIn, error) {
	rows, err := q.db.QueryContext(ctx, getCheckInsForWeek, arg.UserID, arg.CreatedAt, arg.CreatedAt2)
	if err != nil {
		return nil, fmt.Errorf("get check-ins for week: %w", err)
	}
	defer rows.Close()

	var items []CheckIn
	for rows.Next() {
		var c CheckIn
		if err := rows.Scan(&c.ID, &c.UserID, &c.HabitID, &c.Status,
			&c.Mood, &c.Energy, &c.Blocker, &c.Note, &c.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan check-in: %w", err)
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

const getAccountabilityStyle = `SELECT user_id, accountability_style
FROM user_settings
WHERE user_id = $1`

func (q *Queries) GetAccountabilityStyle(ctx context.Context, userID uuid.UUID) (UserSetting, error) {
	var s UserSetting
	err := q.db.QueryRowContext(ctx, getAccountabilityStyle, userID).Scan(
		&s.UserID, &s.AccountabilityStyle,
	)
	if err != nil {
		return UserSetting{}, fmt.Errorf("get accountability style: %w", err)
	}
	return s, nil
}

const markProcessed = `INSERT INTO ai_coach_processed_events (event_id, processed_at)
VALUES ($1, NOW())
ON CONFLICT (event_id) DO NOTHING`

func (q *Queries) MarkProcessed(ctx context.Context, eventID uuid.UUID) error {
	_, err := q.db.ExecContext(ctx, markProcessed, eventID)
	if err != nil {
		return fmt.Errorf("mark processed: %w", err)
	}
	return nil
}

const isProcessed = `SELECT EXISTS(SELECT 1 FROM ai_coach_processed_events WHERE event_id = $1)`

func (q *Queries) IsProcessed(ctx context.Context, eventID uuid.UUID) (bool, error) {
	var exists bool
	err := q.db.QueryRowContext(ctx, isProcessed, eventID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("is processed: %w", err)
	}
	return exists, nil
}

// DBTX is the common interface for database operations.
type DBTX interface {
	ExecContext(context.Context, string, ...interface{}) (sql.Result, error)
	QueryContext(context.Context, string, ...interface{}) (*sql.Rows, error)
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

// NewWithTx creates a Queries using the given DBTX.
func NewWithTx(db DBTX) *Queries {
	return &Queries{db: db.(*sql.DB)}
}
