package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/scheduler"
	"github.com/zeromicro/go-zero/core/logx"
)

// Clock abstracts time.Now for testability.
type Clock interface {
	Now() time.Time
}

// realClock returns the actual wall-clock time.
type realClock struct{}

func (realClock) Now() time.Time { return time.Now() }

// EventsHandler consumes domain events from the growth.events topic and
// manages reminder_queue rows accordingly.
type EventsHandler struct {
	repo  *repository.Repository
	pub   Publisher
	clock Clock
}

// Publisher publishes event envelopes. Declared by the consumer package so
// fakes can be provided in tests.
type Publisher interface {
	Publish(ctx context.Context, env events.Envelope) error
}

// NewEventsHandler creates a handler with the given dependencies. If clock is
// nil, the real wall-clock is used.
func NewEventsHandler(repo *repository.Repository, pub Publisher, clock Clock) *EventsHandler {
	if clock == nil {
		clock = realClock{}
	}
	return &EventsHandler{repo: repo, pub: pub, clock: clock}
}

// Consume is the kq.ConsumeHandler callback. The key parameter is unused.
// Errors are returned so kq retries on transient failures; validation errors
// return nil after logging.
func (h *EventsHandler) Consume(ctx context.Context, _ string, raw string) error {
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
		logx.WithContext(ctx).Infof("duplicate event %s, skipping", env.EventID)
		return nil
	}

	var handlerErr error
	switch events.EventType(env.EventType) {
	case events.TypeCheckInCreated:
		handlerErr = h.onCheckInCreated(ctx, env)
	case events.TypeUserOnboarded:
		handlerErr = h.onUserOnboarded(ctx, env)
	case events.TypeSettingsChanged:
		handlerErr = h.onSettingsChanged(ctx, env)
	case events.TypeCheckInFeedbackGenerated:
		handlerErr = h.onCheckInFeedbackGenerated(ctx, env)
	default:
		logx.WithContext(ctx).Infof("unhandled event type %s", env.EventType)
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

func (h *EventsHandler) onCheckInCreated(ctx context.Context, env events.Envelope) error {
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

	now := h.clock.Now()

	// Look up user timezone for correct date comparison.
	rc, err := h.repo.Reminders.GetContext(ctx, userID)
	if err != nil {
		logx.WithContext(ctx).Errorf("get context for cancel: %v", err)
	} else {
		// Cancel today's missed_check_in reminder since user just checked in.
		if err := h.repo.Reminders.CancelPendingForDate(ctx, userID, "missed_check_in", now, rc.Timezone); err != nil {
			logx.WithContext(ctx).Errorf("cancel missed_check_in: %v", err)
		}
	}

	// If streak is a milestone, enqueue encouragement at now+2m.
	streakMilestones := map[int32]bool{7: true, 14: true, 30: true, 60: true, 100: true}
	if streakMilestones[p.Streak] {
		_, err := h.repo.Reminders.Enqueue(ctx, userID, "encouragement",
			now.Add(2*time.Minute),
			map[string]any{"streak": p.Streak, "habitName": p.HabitName},
		)
		if err != nil {
			return fmt.Errorf("enqueue encouragement: %w", err)
		}
	}

	return nil
}

func (h *EventsHandler) onUserOnboarded(ctx context.Context, env events.Envelope) error {
	var p events.UserOnboarded
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.WithContext(ctx).Errorf("unmarshal UserOnboarded: %v", err)
		return nil
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid userID %q: %v", p.UserID, err)
		return nil
	}

	return h.scheduleRemindersFromSettings(ctx, userID)
}

func (h *EventsHandler) onSettingsChanged(ctx context.Context, env events.Envelope) error {
	var p events.SettingsChanged
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.WithContext(ctx).Errorf("unmarshal SettingsChanged: %v", err)
		return nil
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid userID %q: %v", p.UserID, err)
		return nil
	}

	return h.scheduleRemindersFromSettings(ctx, userID)
}

func (h *EventsHandler) onCheckInFeedbackGenerated(ctx context.Context, env events.Envelope) error {
	var p events.CheckInFeedbackGenerated
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.WithContext(ctx).Errorf("unmarshal CheckInFeedbackGenerated: %v", err)
		return nil
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid userID %q: %v", p.UserID, err)
		return nil
	}

	_, err = h.repo.Notifications.CreateNotification(ctx, db.CreateNotificationParams{
		Title:    "Coach feedback",
		Message:  p.Content,
		ItemType: "ai_feedback",
		UserID:   userID,
	})
	if err != nil {
		return fmt.Errorf("create ai_feedback notification: %w", err)
	}

	return nil
}

// scheduleRemindersFromSettings cancels pending habit_reminder and weekly_review
// for the user, then enqueues the next occurrence based on their timezone and
// check_in_time preferences.
func (h *EventsHandler) scheduleRemindersFromSettings(ctx context.Context, userID uuid.UUID) error {
	rc, err := h.repo.Reminders.GetContext(ctx, userID)
	if err != nil {
		return fmt.Errorf("get reminder context: %w", err)
	}

	now := h.clock.Now()

	// Cancel existing pending reminders so we can reschedule.
	if err := h.repo.Reminders.CancelPendingForDate(ctx, userID, "habit_reminder", now, rc.Timezone); err != nil {
		logx.WithContext(ctx).Errorf("cancel habit_reminder: %v", err)
	}
	if err := h.repo.Reminders.CancelPendingForDate(ctx, userID, "weekly_review", now, rc.Timezone); err != nil {
		logx.WithContext(ctx).Errorf("cancel weekly_review: %v", err)
	}

	// Schedule next habit_reminder at user's check_in_time in their timezone.
	if rc.HabitReminders && rc.OnboardingCompleted {
		next, err := scheduler.NextOccurrence(now, rc.Timezone, rc.CheckInTime)
		if err != nil {
			logx.WithContext(ctx).Errorf("next occurrence: %v", err)
		} else {
			if _, err := h.repo.Reminders.Enqueue(ctx, userID, "habit_reminder", next, nil); err != nil {
				return fmt.Errorf("enqueue habit_reminder: %w", err)
			}
		}
	}

	// Schedule next weekly_review on Sunday 18:00 local.
	nextSun, err := scheduler.NextWeekday(now, rc.Timezone, time.Sunday, 18, 0)
	if err != nil {
		logx.WithContext(ctx).Errorf("next weekday: %v", err)
	} else {
		if _, err := h.repo.Reminders.Enqueue(ctx, userID, "weekly_review", nextSun, nil); err != nil {
			return fmt.Errorf("enqueue weekly_review: %w", err)
		}
	}

	return nil
}
