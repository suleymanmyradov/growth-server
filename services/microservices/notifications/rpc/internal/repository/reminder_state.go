package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"go.opentelemetry.io/otel"
)

// ReminderStateRepo wraps sqlc-generated queries for the reminder_state table.
type ReminderStateRepo struct {
	db *db.Queries
}

// NewReminderStateRepo returns a repo backed by the given sqlc Queries.
func NewReminderStateRepo(q *db.Queries) *ReminderStateRepo {
	return &ReminderStateRepo{db: q}
}

// WithTx returns a new ReminderStateRepo backed by the given transaction.
func (r *ReminderStateRepo) WithTx(tx pgx.Tx) *ReminderStateRepo {
	return &ReminderStateRepo{db: r.db.WithTx(tx)}
}

// Get returns the reminder state for a user.
func (r *ReminderStateRepo) Get(ctx context.Context, userID uuid.UUID) (db.ReminderState, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "ReminderStateRepo.Get")
	defer span.End()
	return r.db.GetReminderState(ctx, userID)
}

// UpsertSettings updates timezone, check_in_time, and habit_reminders from a settings_changed event.
func (r *ReminderStateRepo) UpsertSettings(ctx context.Context, userID uuid.UUID, timezone string, checkInTime pgtype.Time, habitReminders bool) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "ReminderStateRepo.UpsertSettings")
	defer span.End()
	return r.db.UpsertReminderStateSettings(ctx, userID, timezone, checkInTime, habitReminders)
}

// SetOnboardingCompleted marks the user as onboarded.
func (r *ReminderStateRepo) SetOnboardingCompleted(ctx context.Context, userID uuid.UUID) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "ReminderStateRepo.SetOnboardingCompleted")
	defer span.End()
	return r.db.SetOnboardingCompleted(ctx, userID)
}

// IncrementHabitCount increases the active habit count by 1.
func (r *ReminderStateRepo) IncrementHabitCount(ctx context.Context, userID uuid.UUID) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "ReminderStateRepo.IncrementHabitCount")
	defer span.End()
	return r.db.IncrementHabitCount(ctx, userID)
}

// DecrementHabitCount decreases the active habit count by 1 (min 0).
func (r *ReminderStateRepo) DecrementHabitCount(ctx context.Context, userID uuid.UUID) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "ReminderStateRepo.DecrementHabitCount")
	defer span.End()
	return r.db.DecrementHabitCount(ctx, userID)
}

// BumpCheckInCountToday increments today's check-in count (resets on new day).
func (r *ReminderStateRepo) BumpCheckInCountToday(ctx context.Context, userID uuid.UUID) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "ReminderStateRepo.BumpCheckInCountToday")
	defer span.End()
	return r.db.BumpCheckInCountToday(ctx, userID)
}

// Delete removes the reminder state for a user (used on account deletion).
func (r *ReminderStateRepo) Delete(ctx context.Context, userID uuid.UUID) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "ReminderStateRepo.Delete")
	defer span.End()
	return r.db.DeleteReminderState(ctx, userID)
}
