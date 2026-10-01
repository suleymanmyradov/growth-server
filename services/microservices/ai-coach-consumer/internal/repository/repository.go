package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/repository/db"
)

type Repository struct {
	queries *db.Queries
}

func NewRepository(q *db.Queries) *Repository {
	return &Repository{queries: q}
}

// Queries exposes the raw query layer for tx-scoped use by the outbox store.
func (r *Repository) Queries() *db.Queries {
	return r.queries
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

// GetCheckInsForWeek returns check-ins for a user in the given occurred_at
// range from the ai_coach_check_ins read model.
func (r *Repository) GetCheckInsForWeek(ctx context.Context, userID uuid.UUID, start, end time.Time) ([]db.CoachCheckIn, error) {
	if r.queries == nil {
		return nil, nil
	}
	return r.queries.GetCheckInsForWeek(ctx, db.GetCheckInsForWeekParams{
		UserID:      userID,
		OccurredAt:  start,
		OccurredAt2: end,
	})
}

// GetCheckInsForDate returns all check-ins for a user on a specific local
// date (YYYY-MM-DD) from the ai_coach_check_ins read model. Used by the
// daily coach digest.
func (r *Repository) GetCheckInsForDate(ctx context.Context, userID uuid.UUID, localDate string) ([]db.CoachCheckIn, error) {
	if r.queries == nil {
		return nil, nil
	}
	return r.queries.GetCheckInsForDate(ctx, userID, localDate)
}

// UpsertCoachCheckIn mirrors a check_in_created event into the read model.
func (r *Repository) UpsertCoachCheckIn(ctx context.Context, arg db.UpsertCoachCheckInParams) error {
	if r.queries == nil {
		return nil
	}
	return r.queries.UpsertCoachCheckIn(ctx, arg)
}

// GetAccountabilityStyle returns the user's accountability style from the
// ai_coach_profiles read model.
func (r *Repository) GetAccountabilityStyle(ctx context.Context, userID uuid.UUID) (string, error) {
	if r.queries == nil {
		return "", nil
	}
	s, err := r.queries.GetAccountabilityStyle(ctx, userID)
	if err != nil {
		return "", fmt.Errorf("get accountability style: %w", err)
	}
	return s, nil
}

// UpsertCoachProfile mirrors a coaching_profile_changed event into the read
// model.
func (r *Repository) UpsertCoachProfile(ctx context.Context, userID uuid.UUID, style string) error {
	if r.queries == nil {
		return nil
	}
	return r.queries.UpsertCoachProfile(ctx, userID, style)
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
	if r.queries == nil {
		return nil
	}
	return r.queries.MarkProcessed(ctx, eventID)
}

// EnqueueEvent writes an outbox row for a pending event publication (P1).
func (r *Repository) EnqueueEvent(ctx context.Context, eventID uuid.UUID, eventType string, payload []byte, occurredAt time.Time) error {
	if r.queries == nil {
		return nil
	}
	return r.queries.EnqueueEvent(ctx, eventID, eventType, payload, pgtypeTimestamptz(occurredAt))
}

// DeleteAIFeedbackByUser removes all AI feedback rows for a user (used on account deletion).
func (r *Repository) DeleteAIFeedbackByUser(ctx context.Context, userID uuid.UUID) error {
	if r.queries == nil {
		return nil
	}
	return r.queries.DeleteAIFeedbackByUser(ctx, userID)
}

// DeleteCoachCheckInsByUser removes mirrored check-in rows (user deletion).
func (r *Repository) DeleteCoachCheckInsByUser(ctx context.Context, userID uuid.UUID) error {
	if r.queries == nil {
		return nil
	}
	return r.queries.DeleteCoachCheckInsByUser(ctx, userID)
}

// DeleteCoachProfileByUser removes the mirrored coaching profile (user deletion).
func (r *Repository) DeleteCoachProfileByUser(ctx context.Context, userID uuid.UUID) error {
	if r.queries == nil {
		return nil
	}
	return r.queries.DeleteCoachProfileByUser(ctx, userID)
}

func pgtypeTimestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}
