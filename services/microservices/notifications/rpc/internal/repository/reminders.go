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

// RemindersRepo wraps sqlc-generated queries for the reminders table.
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
func (r *RemindersRepo) Enqueue(ctx context.Context, userID uuid.UUID, reminderType string, scheduledAt time.Time, metadata any) (db.EnqueueReminderRow, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "RemindersRepo.Enqueue")
	defer span.End()

	raw, err := json.Marshal(metadata)
	if err != nil {
		return db.EnqueueReminderRow{}, fmt.Errorf("marshal metadata: %w", err)
	}
	return r.db.EnqueueReminder(ctx, userID, reminderType, pgtype.Timestamptz{Time: scheduledAt, Valid: true}, raw)
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
	return r.db.CancelPendingReminderForDate(ctx, userID, reminderType, pgDay, timezone)
}

// CancelPendingByType deletes all unsent reminders of the given type for a
// user. Used when the user disables a notification preference so no further
// reminders of that type fire until re-enabled. Returns the number of rows
// deleted.
func (r *RemindersRepo) CancelPendingByType(ctx context.Context, userID uuid.UUID, reminderType string) (int64, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "RemindersRepo.CancelPendingByType")
	defer span.End()
	return r.db.CancelPendingByType(ctx, userID, reminderType)
}

// ClaimDue claims up to limit due, unclaimed, unsent reminders by setting
// claimed_at (a lease). It does NOT mark them sent — that happens only after
// successful Kafka publish via MarkSent. Uses FOR UPDATE SKIP LOCKED so
// multiple instances don't claim the same rows.
func (r *RemindersRepo) ClaimDue(ctx context.Context, limit int32) ([]db.ClaimDueRemindersRow, error) {
	return r.ClaimDueReminders(ctx, limit)
}

// ClaimDueReminders delegates to the sqlc-generated query. Claims reminders
// by setting claimed_at (lease), not sent_at.
func (r *RemindersRepo) ClaimDueReminders(ctx context.Context, limit int32) ([]db.ClaimDueRemindersRow, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "RemindersRepo.ClaimDue")
	defer span.End()

	return r.db.ClaimDueReminders(ctx, limit)
}

// ReleaseStaleClaims releases claims older than leaseMinutes so they can be
// re-claimed by the next tick. Handles scheduler crashes: a reminder that
// was claimed but never acked (sent_at still NULL) becomes claimable again
// after the lease expires.
func (r *RemindersRepo) ReleaseStaleClaims(ctx context.Context, leaseMinutes int32) (int64, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "RemindersRepo.ReleaseStaleClaims")
	defer span.End()
	return r.db.ReleaseStaleClaims(ctx, leaseMinutes)
}

// GetPendingByUser returns all unsent reminders for the given user.
func (r *RemindersRepo) GetPendingByUser(ctx context.Context, userID uuid.UUID) ([]db.GetPendingByUserRow, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "RemindersRepo.GetPendingByUser")
	defer span.End()

	return r.db.GetPendingByUser(ctx, userID)
}

// MarkSent marks a single reminder as sent by ID (ack after successful publish).
func (r *RemindersRepo) MarkSent(ctx context.Context, id uuid.UUID) (db.MarkReminderSentRow, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "RemindersRepo.MarkSent")
	defer span.End()

	return r.db.MarkReminderSent(ctx, id)
}

// IsDuplicateEvent returns true when the event has already been processed.
// This is a read-only SELECT and does not mutate the processed_events table.
func (r *RemindersRepo) IsDuplicateEvent(ctx context.Context, eventID uuid.UUID) bool {
	processed, err := r.db.IsEventProcessed(ctx, eventID.String())
	if err != nil {
		return false
	}
	return processed
}

// DeleteByUser removes all reminders for a user (used on account deletion).
func (r *RemindersRepo) DeleteByUser(ctx context.Context, userID uuid.UUID) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "RemindersRepo.DeleteByUser")
	defer span.End()
	return r.db.DeleteRemindersByUser(ctx, userID)
}
