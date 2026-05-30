package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"go.opentelemetry.io/otel"
)

// RemindersRepo wraps sqlc-generated queries for the reminder_queue table.
type RemindersRepo struct {
	db *db.Queries
}

// NewRemindersRepo returns a repo backed by the given sqlc Queries.
func NewRemindersRepo(q *db.Queries) *RemindersRepo {
	return &RemindersRepo{db: q}
}

// WithTx returns a new RemindersRepo backed by the given transaction.
func (r *RemindersRepo) WithTx(tx pgx.Tx) *RemindersRepo {
	return &RemindersRepo{db: r.db.WithTx(tx)}
}

// Enqueue inserts or updates a pending reminder for the given user, type, and
// scheduled date. The partial unique index ensures at most one pending row per
// (user_id, type, scheduled_at::date).
func (r *RemindersRepo) Enqueue(ctx context.Context, userID uuid.UUID, reminderType string, scheduledAt time.Time, metadata any) (db.ReminderQueue, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "RemindersRepo.Enqueue")
	defer span.End()

	raw, err := json.Marshal(metadata)
	if err != nil {
		return db.ReminderQueue{}, fmt.Errorf("marshal metadata: %w", err)
	}
	return r.db.EnqueueReminder(ctx, userID, db.ReminderType(reminderType), pgtype.Timestamptz{Time: scheduledAt, Valid: true}, raw)
}

// CancelPendingForDate deletes the unsent reminder for (user, type, day).
// The timezone parameter is used to compare dates in the user's local time.
func (r *RemindersRepo) CancelPendingForDate(ctx context.Context, userID uuid.UUID, reminderType string, day time.Time, timezone string) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "RemindersRepo.CancelPendingForDate")
	defer span.End()

	pgDay := pgtype.Date{Valid: true}
	if err := pgDay.Scan(day); err != nil {
		return fmt.Errorf("convert day to pgtype.Date: %w", err)
	}
	return r.db.CancelPendingReminderForDate(ctx, userID, db.ReminderType(reminderType), pgDay, timezone)
}

// ClaimDue selects up to limit unsent reminders that are due, marks them sent,
// and returns them. Uses FOR UPDATE SKIP LOCKED so multiple instances don't
// claim the same rows.
func (r *RemindersRepo) ClaimDue(ctx context.Context, limit int32) ([]db.ReminderQueue, error) {
	return r.ClaimDueReminders(ctx, limit)
}

// ClaimDueReminders delegates to the sqlc-generated query.
func (r *RemindersRepo) ClaimDueReminders(ctx context.Context, limit int32) ([]db.ReminderQueue, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "RemindersRepo.ClaimDue")
	defer span.End()

	return r.db.ClaimDueReminders(ctx, limit)
}

// GetPendingByUser returns all unsent reminders for the given user.
func (r *RemindersRepo) GetPendingByUser(ctx context.Context, userID uuid.UUID) ([]db.ReminderQueue, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "RemindersRepo.GetPendingByUser")
	defer span.End()

	return r.db.GetPendingByUser(ctx, userID)
}

// GetContext loads the user settings and habit/check-in state needed to decide
// whether a reminder should fire.
func (r *RemindersRepo) GetContext(ctx context.Context, userID uuid.UUID) (db.GetReminderContextRow, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "RemindersRepo.GetContext")
	defer span.End()

	return r.db.GetReminderContext(ctx, userID)
}

// MarkSent marks a single reminder as sent by ID.
func (r *RemindersRepo) MarkSent(ctx context.Context, id uuid.UUID) (db.ReminderQueue, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "RemindersRepo.MarkSent")
	defer span.End()

	return r.db.MarkReminderSent(ctx, id)
}

// IsDuplicateEvent returns true when the event has already been processed.
// This is a read-only SELECT and does not mutate the processed_events table.
func (r *RemindersRepo) IsDuplicateEvent(ctx context.Context, eventID uuid.UUID) bool {
	processed, err := r.db.IsEventProcessed(ctx, eventID)
	if err != nil {
		return false
	}
	return processed
}
