package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
)

type HabitStateRepo struct {
	db *db.Queries
}

func NewHabitStateRepo(q *db.Queries) *HabitStateRepo {
	return &HabitStateRepo{db: q}
}

func (r *HabitStateRepo) Create(ctx context.Context, userID, habitID uuid.UUID, name string) (db.NotificationHabitState, error) {
	return r.db.CreateNotificationHabitState(ctx, userID, habitID, name)
}

func (r *HabitStateRepo) UpdateCheckIn(ctx context.Context, userID, habitID uuid.UUID, name string, streak int32, localDate time.Time) (db.NotificationHabitState, error) {
	date := pgtype.Date{Time: localDate, Valid: true}
	return r.db.UpsertNotificationHabitState(ctx, db.UpsertNotificationHabitStateParams{
		UserID:               userID,
		HabitID:              habitID,
		HabitName:            name,
		CurrentStreak:        streak,
		LastCheckInLocalDate: date,
	})
}

func (r *HabitStateRepo) Delete(ctx context.Context, userID, habitID uuid.UUID) error {
	return r.db.DeleteNotificationHabitState(ctx, userID, habitID)
}

func (r *HabitStateRepo) ListAtRisk(ctx context.Context, userID uuid.UUID, localDate time.Time, limit int32) ([]db.NotificationHabitState, error) {
	date := pgtype.Date{Time: localDate, Valid: true}
	return r.db.ListHabitsAtStreakRisk(ctx, userID, date, limit)
}

func (r *HabitStateRepo) DeleteByUser(ctx context.Context, userID uuid.UUID) error {
	return r.db.DeleteNotificationHabitStatesByUser(ctx, userID)
}
