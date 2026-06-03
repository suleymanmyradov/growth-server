package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/consumer/internal/repository/db"
)

type Repository struct {
	queries *db.Queries
}

func NewRepository(q *db.Queries) *Repository {
	return &Repository{queries: q}
}

// WithTx returns a new Repository backed by the given transaction.
func (r *Repository) WithTx(tx pgx.Tx) *Repository {
	return NewRepository(db.NewWithTx(tx))
}

// InsertAIFeedback inserts a new AI feedback row.
func (r *Repository) InsertAIFeedback(ctx context.Context, arg db.InsertAIFeedbackParams) error {
	if r.queries == nil {
		return nil
	}
	return r.queries.InsertAIFeedback(ctx, arg)
}

// GetCheckInsForWeek returns check-ins for a user in the given date range.
func (r *Repository) GetCheckInsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.CheckIn, error) {
	if r.queries == nil {
		return nil, nil
	}
	return r.queries.GetCheckInsForWeek(ctx, db.GetCheckInsForWeekParams{
		UserID:     userID,
		CreatedAt:  start,
		CreatedAt2: end,
	})
}

// GetAccountabilityStyle returns the user's accountability style setting.
func (r *Repository) GetAccountabilityStyle(ctx context.Context, userID uuid.UUID) (string, error) {
	if r.queries == nil {
		return "", nil
	}
	s, err := r.queries.GetAccountabilityStyle(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get accountability style: %w", err)
	}
	return s.AccountabilityStyle, nil
}

// IsProcessed checks if an event has already been processed.
func (r *Repository) IsProcessed(ctx context.Context, eventID uuid.UUID) (bool, error) {
	if r.queries == nil {
		return false, nil
	}
	return r.queries.IsProcessed(ctx, eventID)
}

// MarkProcessed marks an event as processed for idempotency.
func (r *Repository) MarkProcessed(ctx context.Context, eventID uuid.UUID) error {
	return r.queries.MarkProcessed(ctx, eventID)
}
