package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"go.opentelemetry.io/otel"
)

// PreferencesRepo wraps notification_preferences queries.
type PreferencesRepo struct {
	db *db.Queries
}

func NewPreferencesRepo(q *db.Queries) *PreferencesRepo {
	return &PreferencesRepo{db: q}
}

// WithTx returns a new PreferencesRepo backed by the given transaction.
func (r *PreferencesRepo) WithTx(tx pgx.Tx) *PreferencesRepo {
	return &PreferencesRepo{db: r.db.WithTx(tx)}
}

// Get returns the user's notification preferences, or a zero-value row with
// all defaults enabled if no row exists yet (first access).
func (r *PreferencesRepo) Get(ctx context.Context, userID uuid.UUID) (db.NotificationPreference, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "PreferencesRepo.Get")
	defer span.End()
	pref, err := r.db.GetNotificationPreferences(ctx, userID)
	if err == nil {
		return pref, nil
	}
	if err != pgx.ErrNoRows {
		return db.NotificationPreference{}, err
	}
	return db.NotificationPreference{
		UserID:             userID,
		EmailNotifications: true,
		PushNotifications:  true,
		HabitReminders:     true,
		GoalReminders:      true,
	}, nil
}

// Upsert inserts or updates the user's notification preferences.
func (r *PreferencesRepo) Upsert(ctx context.Context, arg db.UpsertNotificationPreferencesParams) (db.NotificationPreference, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "PreferencesRepo.Upsert")
	defer span.End()
	return r.db.UpsertNotificationPreferences(ctx, arg)
}

// DeleteByUser removes the user's preferences row (used on account deletion).
func (r *PreferencesRepo) DeleteByUser(ctx context.Context, userID uuid.UUID) error {
	ctx, span := otel.Tracer("notifications").Start(ctx, "PreferencesRepo.DeleteByUser")
	defer span.End()
	return r.db.DeleteNotificationPreferences(ctx, userID)
}
