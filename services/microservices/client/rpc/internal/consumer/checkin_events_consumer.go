package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

// missedDayThreshold is the number of consecutive missed check-ins after
// which a recovery plan-adjustment suggestion is auto-created.
const missedDayThreshold = 3

// CheckInEventsHandler consumes CheckInCreated events from the growth.events
// topic and maintains the habit_missed_streaks read model. When a habit's
// consecutive missed days cross missedDayThreshold, it auto-creates a
// plan_adjustment suggestion with source='missed_day_recovery' and
// type='reduce_difficulty'.
type CheckInEventsHandler struct {
	repo *repository.Repository
	dbq  *db.Queries
}

// NewCheckInEventsHandler creates a handler for missed-day recovery.
func NewCheckInEventsHandler(repo *repository.Repository, dbq *db.Queries) *CheckInEventsHandler {
	return &CheckInEventsHandler{repo: repo, dbq: dbq}
}

// Consume is the kq.ConsumeHandler callback.
func (h *CheckInEventsHandler) Consume(ctx context.Context, _ string, raw string) error {
	var env events.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		logx.WithContext(ctx).Errorf("invalid envelope: %v", err)
		return nil
	}

	// Only handle CheckInCreated events; ignore everything else.
	if events.EventType(env.EventType) != events.TypeCheckInCreated {
		return nil
	}

	// Dedup via processed_events so redeliveries don't double-process.
	eventID, err := uuid.Parse(env.EventID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid event ID %q: %v", env.EventID, err)
		return nil
	}
	processed, err := h.dbq.IsClientEventProcessed(ctx, eventID.String())
	if err != nil {
		logx.WithContext(ctx).Errorf("check processed_events: %v", err)
	} else if processed {
		logx.WithContext(ctx).Infof("duplicate event %s, skipping", env.EventID)
		return nil
	}

	if err := h.onCheckInCreated(ctx, env); err != nil {
		return err
	}

	if err := h.dbq.MarkClientEventProcessed(ctx, eventID.String()); err != nil {
		logx.WithContext(ctx).Errorf("mark event %s processed: %v", env.EventID, err)
	}
	return nil
}

func (h *CheckInEventsHandler) onCheckInCreated(ctx context.Context, env events.Envelope) error {
	var p events.CheckInCreated
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.WithContext(ctx).Errorf("unmarshal CheckInCreated: %v", err)
		return nil
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid userID %q: %v", p.UserID, err)
		return nil
	}
	habitID, err := uuid.Parse(p.HabitID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid habitID %q: %v", p.HabitID, err)
		return nil
	}

	if p.Status == "completed" {
		return h.handleCompleted(ctx, habitID, userID)
	}
	if p.Status == "missed" {
		return h.handleMissed(ctx, habitID, userID, p.HabitName)
	}
	return nil
}

// handleCompleted resets the missed streak to 0.
func (h *CheckInEventsHandler) handleCompleted(ctx context.Context, habitID, userID uuid.UUID) error {
	today := pgtype.Date{Time: time.Now().UTC(), Valid: true}
	_, err := h.repo.HabitMissedStreaks.ResetHabitMissedStreak(ctx, habitID, userID, today)
	if err != nil {
		// ResetHabitMissedStreak returns no rows if no streak row exists yet,
		// which is fine — a completed check-in with no prior missed streak is
		// the normal case. Only log unexpected errors.
		logx.WithContext(ctx).Infof("reset missed streak for habit %s (may not exist yet): %v", habitID, err)
	}
	return nil
}

// handleMissed increments the missed streak and auto-creates a recovery
// suggestion when the threshold is crossed.
func (h *CheckInEventsHandler) handleMissed(ctx context.Context, habitID, userID uuid.UUID, habitName string) error {
	streak, err := h.repo.HabitMissedStreaks.IncrementHabitMissedStreak(ctx, habitID, userID)
	if err != nil {
		return fmt.Errorf("increment missed streak: %w", err)
	}

	// Only trigger recovery when we cross the threshold AND we haven't already
	// triggered one today (last_recovery_triggered prevents spamming).
	if streak.ConsecutiveMissedDays < missedDayThreshold {
		return nil
	}

	today := time.Now().UTC().Truncate(24 * time.Hour)
	if streak.LastRecoveryTriggered.Valid {
		lastTrigger := streak.LastRecoveryTriggered.Time
		if lastTrigger.After(today.Add(-24 * time.Hour)) {
			// Already triggered a recovery suggestion today (or yesterday),
			// don't spam another one.
			return nil
		}
	}

	// Create a plan adjustment suggestion to reduce difficulty.
	reason := fmt.Sprintf("You missed %s %d days in a row. Let's reduce it to rebuild consistency.", habitName, streak.ConsecutiveMissedDays)
	suggestion := fmt.Sprintf("Scale down %s to a smaller, easier version for the next 5 days. Focus on showing up, not on volume.", habitName)

	metadata, _ := json.Marshal(map[string]string{
		"source":           "missed_day_recovery",
		"consecutive_days": fmt.Sprintf("%d", streak.ConsecutiveMissedDays),
		"habit_name":       habitName,
	})

	_, err = h.repo.PlanAdjustmentSuggestions.CreatePlanAdjustmentSuggestion(ctx, db.CreatePlanAdjustmentSuggestionParams{
		UserID:         userID,
		HabitID:        uuid.NullUUID{UUID: habitID, Valid: true},
		Source:         "missed_day_recovery",
		AdjustmentType: "reduce_difficulty",
		Reason:         reason,
		Suggestion:     suggestion,
		Metadata:       metadata,
	})
	if err != nil {
		// The unique index on (user_id, source, adjustment_type, habit_id, week_start)
		// means a duplicate for the same week will upsert — not an error.
		// But if week_start is NULL, the index treats all NULLs as distinct,
		// so we could get duplicates. That's acceptable for now.
		logx.WithContext(ctx).Errorf("failed to create missed-day recovery suggestion: %v", err)
		// Non-fatal: don't block the consumer for this.
		return nil
	}

	// Update last_recovery_triggered to today so we don't spam.
	// We use UpsertHabitMissedStreak to update the row.
	todayDate := pgtype.Date{Time: today, Valid: true}
	_, err = h.dbq.UpsertHabitMissedStreak(ctx, db.UpsertHabitMissedStreakParams{
		HabitID:               habitID,
		UserID:                userID,
		ConsecutiveMissedDays: streak.ConsecutiveMissedDays,
		LastCompletedDate:     streak.LastCompletedDate,
		LastRecoveryTriggered: todayDate,
	})
	if err != nil {
		logx.WithContext(ctx).Errorf("failed to update last_recovery_triggered: %v", err)
	}

	logx.WithContext(ctx).Infof("missed-day recovery triggered for user %s habit %s (%d consecutive misses)",
		userID, habitID, streak.ConsecutiveMissedDays)
	return nil
}
