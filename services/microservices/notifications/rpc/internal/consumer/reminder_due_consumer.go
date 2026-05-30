package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/scheduler"
	"github.com/zeromicro/go-zero/core/logx"
)

// ReminderDueHandler consumes events from the growth.reminder.due topic and
// materializes notification rows, then enqueues follow-up reminders.
type ReminderDueHandler struct {
	repo  *repository.Repository
	clock Clock
}

// NewReminderDueHandler creates a handler with the given dependencies.
func NewReminderDueHandler(repo *repository.Repository, clock Clock) *ReminderDueHandler {
	if clock == nil {
		clock = realClock{}
	}
	return &ReminderDueHandler{repo: repo, clock: clock}
}

// Consume is the kq.ConsumeHandler callback for the reminder.due topic.
func (h *ReminderDueHandler) Consume(ctx context.Context, _ string, raw string) error {
	var env events.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		logx.WithContext(ctx).Errorf("invalid envelope: %v", err)
		return nil
	}

	eventID, err := uuid.Parse(env.EventID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid event ID %q: %v", env.EventID, err)
		return nil
	}

	if h.repo.ProcessedEvents != nil && h.repo.ProcessedEvents.IsProcessed(ctx, eventID) {
		return nil
	}

	var p events.ReminderDue
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.WithContext(ctx).Errorf("unmarshal ReminderDue: %v", err)
		return nil
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid userID %q: %v", p.UserID, err)
		return nil
	}

	var handlerErr error
	switch p.Type {
	case "habit_reminder":
		handlerErr = h.onHabitReminder(ctx, userID, p)
	case "missed_check_in":
		handlerErr = h.onMissedCheckIn(ctx, userID, p)
	case "weekly_review":
		handlerErr = h.onWeeklyReview(ctx, userID, p)
	case "encouragement":
		handlerErr = h.onEncouragement(ctx, userID, p)
	default:
		logx.WithContext(ctx).Infof("unhandled reminder type %s", p.Type)
		return nil
	}

	if handlerErr != nil {
		return handlerErr
	}

	// Mark event as processed only after successful handling.
	if h.repo.ProcessedEvents != nil {
		if err := h.repo.ProcessedEvents.Mark(ctx, eventID); err != nil {
			logx.WithContext(ctx).Errorf("mark event %s processed: %v", env.EventID, err)
		}
	}
	return nil
}

func (h *ReminderDueHandler) onHabitReminder(ctx context.Context, userID uuid.UUID, _ events.ReminderDue) error {
	rc, err := h.repo.Reminders.GetContext(ctx, userID)
	if err != nil {
		return fmt.Errorf("get reminder context: %w", err)
	}

	if rc.ActiveHabitCount == 0 || !rc.HabitReminders {
		return nil
	}

	_, err = h.repo.Notifications.CreateNotification(ctx, "Time to check in", fmt.Sprintf("You have %d habits to check in on today", rc.ActiveHabitCount), "habit_reminder", userID)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

	now := h.clock.Now()

	// Enqueue tomorrow's habit_reminder at user's check_in_time.
	next, err := scheduler.NextOccurrence(now, rc.Timezone, rc.CheckInTime)
	if err != nil {
		logx.WithContext(ctx).Errorf("next occurrence: %v", err)
	} else {
		if _, err := h.repo.Reminders.Enqueue(ctx, userID, "habit_reminder", next, nil); err != nil {
			logx.WithContext(ctx).Errorf("enqueue next habit_reminder: %v", err)
		}
	}

	// Enqueue today's missed_check_in at now + 2h.
	if _, err := h.repo.Reminders.Enqueue(ctx, userID, "missed_check_in",
		now.Add(2*time.Hour), nil); err != nil {
		logx.WithContext(ctx).Errorf("enqueue missed_check_in: %v", err)
	}

	return nil
}

func (h *ReminderDueHandler) onMissedCheckIn(ctx context.Context, userID uuid.UUID, _ events.ReminderDue) error {
	rc, err := h.repo.Reminders.GetContext(ctx, userID)
	if err != nil {
		return fmt.Errorf("get reminder context: %w", err)
	}

	if rc.CheckedInToday {
		return nil
	}

	_, err = h.repo.Notifications.CreateNotification(ctx, "Missed check-in", "You missed your check-in today. Don't worry, tomorrow is a fresh start!", "missed_check_in", userID)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

	return nil
}

func (h *ReminderDueHandler) onWeeklyReview(ctx context.Context, userID uuid.UUID, _ events.ReminderDue) error {
	_, err := h.repo.Notifications.CreateNotification(ctx, "Weekly review", "Reflect on your week", "weekly_review", userID)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

	// Enqueue next Sunday 18:00 local.
	rc, err := h.repo.Reminders.GetContext(ctx, userID)
	if err != nil {
		return fmt.Errorf("get context for weekly reschedule: %w", err)
	}

	nextSun, err := scheduler.NextWeekday(h.clock.Now(), rc.Timezone, time.Sunday, 18, 0)
	if err != nil {
		logx.WithContext(ctx).Errorf("next weekday: %v", err)
	} else {
		if _, err := h.repo.Reminders.Enqueue(ctx, userID, "weekly_review", nextSun, nil); err != nil {
			logx.WithContext(ctx).Errorf("enqueue weekly_review: %v", err)
		}
	}

	return nil
}

func (h *ReminderDueHandler) onEncouragement(ctx context.Context, userID uuid.UUID, p events.ReminderDue) error {
	var meta map[string]any
	if p.Metadata != "" {
		_ = json.Unmarshal([]byte(p.Metadata), &meta)
	}

	habitName := ""
	streak := int32(0)
	if v, ok := meta["habitName"].(string); ok {
		habitName = v
	}
	if v, ok := meta["streak"].(float64); ok {
		streak = int32(v)
	}

	title := "Great job!"
	msg := "You're building great habits!"
	if habitName != "" && streak > 0 {
		msg = fmt.Sprintf("You've maintained a %d-day streak on %s! Keep it up!", streak, habitName)
	}

	_, err := h.repo.Notifications.CreateNotification(ctx, title, msg, "encouragement", userID)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

	return nil
}
