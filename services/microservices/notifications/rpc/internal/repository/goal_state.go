package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"go.opentelemetry.io/otel"
)

// GoalStateRepo wraps sqlc-generated queries for the notification_goal_state
// table — a local read model maintained from goal lifecycle events so the
// notifications service can reschedule goal_deadline reminders when the user
// toggles the goalReminders preference without calling the client RPC.
type GoalStateRepo struct {
	db *db.Queries
}

func NewGoalStateRepo(q *db.Queries) *GoalStateRepo {
	return &GoalStateRepo{db: q}
}

func (r *GoalStateRepo) WithTx(tx pgx.Tx) *GoalStateRepo {
	return &GoalStateRepo{db: r.db.WithTx(tx)}
}

// Upsert inserts or updates the local goal read model row.
func (r *GoalStateRepo) Upsert(ctx context.Context, goalID, userID uuid.UUID, title string, deadline time.Time, completed bool) (db.NotificationGoalState, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "GoalStateRepo.Upsert")
	defer span.End()

	var dl pgtype.Timestamptz
	if !deadline.IsZero() {
		dl = pgtype.Timestamptz{Time: deadline, Valid: true}
	}
	return r.db.UpsertGoalState(ctx, db.UpsertGoalStateParams{
		GoalID:    goalID,
		UserID:    userID,
		Title:     title,
		Deadline:  dl,
		Completed: completed,
	})
}

func (r *GoalStateRepo) Delete(ctx context.Context, goalID uuid.UUID) error {
	_, err := r.db.DeleteGoalState(ctx, goalID)
	return err
}

func (r *GoalStateRepo) MarkCompleted(ctx context.Context, goalID uuid.UUID) error {
	_, err := r.db.MarkGoalCompleted(ctx, goalID)
	return err
}

// ListUpcomingDeadlines returns the user's active goals with future deadlines,
// ordered by deadline ascending. Used to reschedule reminders on re-enable.
func (r *GoalStateRepo) ListUpcomingDeadlines(ctx context.Context, userID uuid.UUID, now time.Time) ([]db.NotificationGoalState, error) {
	ctx, span := otel.Tracer("notifications").Start(ctx, "GoalStateRepo.ListUpcomingDeadlines")
	defer span.End()

	return r.db.ListUpcomingGoalDeadlines(ctx, userID, pgtype.Timestamptz{Time: now, Valid: true})
}

func (r *GoalStateRepo) DeleteByUser(ctx context.Context, userID uuid.UUID) error {
	return r.db.DeleteGoalStateByUser(ctx, userID)
}
