package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/notifications"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository"
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
// maintains reminder_state + reminder_queue rows accordingly.
type EventsHandler struct {
	repo     *repository.Repository
	pub      Publisher
	clock    Clock
	txRunner *postgres.PgxTxRunner
	dlq      DLQPublisher
}

// DLQPublisher pushes poison messages to a dead-letter topic. Declared here so
// fakes can be provided in tests.
type DLQPublisher interface {
	Publish(ctx context.Context, msg events.DLQMessage) error
}

// Publisher publishes event envelopes. Declared by the consumer package so
// fakes can be provided in tests.
type Publisher interface {
	Publish(ctx context.Context, env events.Envelope) error
}

// NewEventsHandler creates a handler with the given dependencies. If clock is
// nil, the real wall-clock is used. If txRunner is nil, the handler+mark pair
// runs without a transaction (side-effects and the processed_events mark are
// not atomic — a Mark failure may cause duplicate side-effects on redelivery).
// If dlq is nil, poison messages are logged and dropped instead of being
// routed to a dead-letter topic.
func NewEventsHandler(repo *repository.Repository, pub Publisher, clock Clock, txRunner *postgres.PgxTxRunner, dlq DLQPublisher) *EventsHandler {
	if clock == nil {
		clock = realClock{}
	}
	return &EventsHandler{repo: repo, pub: pub, clock: clock, txRunner: txRunner, dlq: dlq}
}

// sendToDLQ publishes a poison message to the DLQ. If no DLQ publisher is
// configured, it logs the rejection instead.
func (h *EventsHandler) sendToDLQ(ctx context.Context, env events.Envelope, raw, reason string) {
	msg := events.DLQMessage{
		Original:    env,
		Raw:         raw,
		Reason:      reason,
		Permanent:   true,
		ServiceName: "notifications",
		OccurredAt:  h.clock.Now(),
	}
	if h.dlq != nil {
		if err := h.dlq.Publish(ctx, msg); err != nil {
			logx.WithContext(ctx).Errorf("failed to publish to DLQ: %v (reason=%s)", err, reason)
		}
	} else {
		logx.WithContext(ctx).Errorf("poison message dropped (no DLQ): reason=%s", reason)
	}
}

// Consume is the kq.ConsumeHandler callback. The key parameter is unused.
// Errors are returned so kq retries on transient failures; validation errors
// return nil after logging.
//
// When a txRunner is configured, the handler dispatch and the processed_events
// marking run inside a single transaction so that a crash or Mark failure
// cannot leave side-effects without a processed_events row — which would cause
// duplicate notifications on redelivery. If the transaction fails, the error
// is returned and kq retries the entire event (the IsProcessed check at the
// top prevents re-processing once Mark commits).
func (h *EventsHandler) Consume(ctx context.Context, _ string, raw string) error {
	var env events.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		h.sendToDLQ(ctx, events.Envelope{}, raw, fmt.Sprintf("invalid envelope JSON: %v", err))
		return nil
	}

	eventID, err := uuid.Parse(env.EventID)
	if err != nil {
		h.sendToDLQ(ctx, env, raw, fmt.Sprintf("invalid event ID %q: %v", env.EventID, err))
		return nil
	}

	if h.repo.ProcessedEvents != nil && h.repo.ProcessedEvents.IsProcessed(ctx, eventID) {
		logx.WithContext(ctx).Infof("duplicate event %s, skipping", env.EventID)
		return nil
	}

	// Run the handler dispatch + processed_events mark atomically when a
	// txRunner is configured. Without a txRunner, fall back to non-atomic
	// execution (handler then mark, Mark failure logged).
	if h.txRunner != nil && h.repo.ProcessedEvents != nil {
		return h.runInTransaction(ctx, env, eventID)
	}

	// Non-transactional fallback (tests / legacy wiring).
	handlerErr := h.dispatch(ctx, h.repo, env)
	if handlerErr != nil {
		return handlerErr
	}
	if h.repo.ProcessedEvents != nil {
		if err := h.repo.ProcessedEvents.Mark(ctx, eventID); err != nil {
			logx.WithContext(ctx).Errorf("mark event %s processed: %v", env.EventID, err)
		}
	}
	return nil
}

// runInTransaction runs the handler dispatch and the processed_events Mark in
// a single transaction. If either fails, the transaction is rolled back and
// the error is returned so kq retries. No RLS user context is set (the
// notifications service operates with service-level access).
func (h *EventsHandler) runInTransaction(ctx context.Context, env events.Envelope, eventID uuid.UUID) error {
	return h.txRunner.Run(ctx, "", func(tx pgx.Tx) error {
		txRepo := repository.NewRepositoryFromTx(tx)

		if err := h.dispatch(ctx, txRepo, env); err != nil {
			return err
		}

		if err := txRepo.ProcessedEvents.Mark(ctx, eventID); err != nil {
			return fmt.Errorf("mark event %s processed: %w", env.EventID, err)
		}
		return nil
	})
}

// dispatch routes the envelope to the appropriate handler method, using the
// provided repo (which may be transaction-scoped).
func (h *EventsHandler) dispatch(ctx context.Context, repo *repository.Repository, env events.Envelope) error {
	switch events.EventType(env.EventType) {
	case events.TypeCheckInCreated:
		return h.onCheckInCreated(ctx, repo, env)
	case events.TypeUserOnboarded:
		return h.onUserOnboarded(ctx, repo, env)
	case events.TypeSettingsChanged:
		return h.onSettingsChanged(ctx, repo, env)
	case events.TypeCheckInFeedbackGenerated:
		return h.onCheckInFeedbackGenerated(ctx, repo, env)
	case events.TypeHabitCreated:
		return h.onHabitCreated(ctx, repo, env)
	case events.TypeHabitDeleted:
		return h.onHabitDeleted(ctx, repo, env)
	case events.TypeUserDeleted:
		return h.onUserDeleted(ctx, repo, env)
	case events.TypeBroadcastNotificationRequested:
		return h.onBroadcastNotificationRequested(ctx, repo, env)
	default:
		logx.WithContext(ctx).Infof("unhandled event type %s", env.EventType)
		return nil
	}
}

func (h *EventsHandler) onCheckInCreated(ctx context.Context, repo *repository.Repository, env events.Envelope) error {
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

	// Bump today's check-in count in the local read model.
	if repo.ReminderState != nil {
		if err := repo.ReminderState.BumpCheckInCountToday(ctx, userID); err != nil {
			logx.WithContext(ctx).Errorf("bump check-in count: %v", err)
		}
	}

	// Look up user timezone for correct date comparison.
	if repo.ReminderState != nil {
		rs, err := repo.ReminderState.Get(ctx, userID)
		if err != nil {
			logx.WithContext(ctx).Errorf("get reminder state for cancel: %v", err)
		} else {
			// Cancel today's missed_check_in reminder since user just checked in.
			if err := repo.Reminders.CancelPendingForDate(ctx, userID, "missed_check_in", now, rs.Timezone); err != nil {
				logx.WithContext(ctx).Errorf("cancel missed_check_in: %v", err)
			}
		}
	}

	// If streak is a milestone, enqueue encouragement at now+2m.
	streakMilestones := map[int32]bool{7: true, 14: true, 30: true, 60: true, 100: true}
	if streakMilestones[p.Streak] {
		_, err := repo.Reminders.Enqueue(ctx, userID, "encouragement",
			now.Add(2*time.Minute),
			map[string]any{"streak": p.Streak, "habitName": p.HabitName},
		)
		if err != nil {
			return fmt.Errorf("enqueue encouragement: %w", err)
		}
	}

	return nil
}

func (h *EventsHandler) onUserOnboarded(ctx context.Context, repo *repository.Repository, env events.Envelope) error {
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

	// Mark onboarding completed in the local read model.
	if repo.ReminderState != nil {
		if err := repo.ReminderState.SetOnboardingCompleted(ctx, userID); err != nil {
			logx.WithContext(ctx).Errorf("set onboarding completed: %v", err)
		}
	}

	return h.scheduleRemindersFromState(ctx, repo, userID)
}

func (h *EventsHandler) onSettingsChanged(ctx context.Context, repo *repository.Repository, env events.Envelope) error {
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

	// Update the local read model with new settings. The habit-reminders flag
	// is owned by the notifications service (notification_preferences table),
	// not the client settings event — so we read it from our own preferences
	// table rather than trusting the event payload.
	if repo.ReminderState != nil {
		var checkInTime pgtype.Time
		if p.CheckInTime != "" {
			if t, err := time.Parse("15:04", p.CheckInTime); err == nil {
				checkInTime = pgtype.Time{Microseconds: (int64(t.Hour())*3600 + int64(t.Minute())*60) * 1_000_000, Valid: true}
			}
		}
		timezone := p.Timezone
		if timezone == "" {
			timezone = "UTC"
		}
		habitReminders := true
		if repo.Preferences != nil {
			if pref, err := repo.Preferences.Get(ctx, userID); err == nil {
				habitReminders = pref.HabitReminders
			}
		}
		if err := repo.ReminderState.UpsertSettings(ctx, userID, timezone, checkInTime, habitReminders); err != nil {
			logx.WithContext(ctx).Errorf("upsert reminder state settings: %v", err)
		}
	}

	return h.scheduleRemindersFromState(ctx, repo, userID)
}

func (h *EventsHandler) onCheckInFeedbackGenerated(ctx context.Context, repo *repository.Repository, env events.Envelope) error {
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

	_, err = repo.Notifications.CreateNotification(ctx, "Coach feedback", p.Content, "ai_feedback", userID)
	if err != nil {
		return fmt.Errorf("create ai_feedback notification: %w", err)
	}

	return nil
}

func (h *EventsHandler) onHabitCreated(ctx context.Context, repo *repository.Repository, env events.Envelope) error {
	var p events.HabitCreated
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.WithContext(ctx).Errorf("unmarshal HabitCreated: %v", err)
		return nil
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid userID %q: %v", p.UserID, err)
		return nil
	}

	if repo.ReminderState != nil {
		if err := repo.ReminderState.IncrementHabitCount(ctx, userID); err != nil {
			return fmt.Errorf("increment habit count: %w", err)
		}
	}
	return nil
}

func (h *EventsHandler) onHabitDeleted(ctx context.Context, repo *repository.Repository, env events.Envelope) error {
	var p events.HabitDeleted
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.WithContext(ctx).Errorf("unmarshal HabitDeleted: %v", err)
		return nil
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid userID %q: %v", p.UserID, err)
		return nil
	}

	if repo.ReminderState != nil {
		if err := repo.ReminderState.DecrementHabitCount(ctx, userID); err != nil {
			return fmt.Errorf("decrement habit count: %w", err)
		}
	}
	return nil
}

func (h *EventsHandler) onUserDeleted(ctx context.Context, repo *repository.Repository, env events.Envelope) error {
	var p events.UserDeleted
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.WithContext(ctx).Errorf("unmarshal UserDeleted: %v", err)
		return nil
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid userID %q: %v", p.UserID, err)
		return nil
	}

	// Clean up all notifications-owned tables for this user.
	if err := repo.Notifications.DeleteByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete notifications: %w", err)
	}
	if err := repo.Reminders.DeleteByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete reminders: %w", err)
	}
	if repo.ReminderState != nil {
		if err := repo.ReminderState.Delete(ctx, userID); err != nil {
			return fmt.Errorf("delete reminder state: %w", err)
		}
	}
	return nil
}

// scheduleRemindersFromState cancels pending habit_reminder and weekly_review
// for the user, then enqueues the next occurrence based on their timezone and
// check_in_time from the local reminder_state read model.
func (h *EventsHandler) scheduleRemindersFromState(ctx context.Context, repo *repository.Repository, userID uuid.UUID) error {
	if repo.ReminderState == nil {
		return nil
	}

	rs, err := repo.ReminderState.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get reminder state: %w", err)
	}

	now := h.clock.Now()

	// Cancel existing pending reminders so we can reschedule.
	if err := repo.Reminders.CancelPendingForDate(ctx, userID, "habit_reminder", now, rs.Timezone); err != nil {
		logx.WithContext(ctx).Errorf("cancel habit_reminder: %v", err)
	}
	if err := repo.Reminders.CancelPendingForDate(ctx, userID, "weekly_review", now, rs.Timezone); err != nil {
		logx.WithContext(ctx).Errorf("cancel weekly_review: %v", err)
	}

	// Schedule next habit_reminder at user's check_in_time in their timezone.
	if rs.HabitReminders && rs.OnboardingCompleted {
		next, err := scheduler.NextOccurrence(now, rs.Timezone, rs.CheckInTime)
		if err != nil {
			logx.WithContext(ctx).Errorf("next occurrence: %v", err)
		} else {
			if _, err := repo.Reminders.Enqueue(ctx, userID, "habit_reminder", next, nil); err != nil {
				return fmt.Errorf("enqueue habit_reminder: %w", err)
			}
		}
	}

	// Schedule next weekly_review on Sunday 18:00 local.
	nextSun, err := scheduler.NextWeekday(now, rs.Timezone, time.Sunday, 18, 0)
	if err != nil {
		logx.WithContext(ctx).Errorf("next weekday: %v", err)
	} else {
		if _, err := repo.Reminders.Enqueue(ctx, userID, "weekly_review", nextSun, nil); err != nil {
			return fmt.Errorf("enqueue weekly_review: %w", err)
		}
	}

	return nil
}

// onBroadcastNotificationRequested handles admin broadcasts: batch-inserts the
// notification for every user id in the chunk. adminway has already resolved
// the audience and chunked the ids, so the consumer only owns the insert
// (notifications table is owned by this service). Errors are returned so kq
// retries the chunk; duplicate delivery is acceptable (idempotency at the
// notification level is not enforced — a retry may create duplicate rows, which
// is an acceptable trade-off for broadcasts vs. tracking a processed_events
// entry per chunk).
func (h *EventsHandler) onBroadcastNotificationRequested(ctx context.Context, repo *repository.Repository, env events.Envelope) error {
	var p events.BroadcastNotificationRequested
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.WithContext(ctx).Errorf("unmarshal BroadcastNotificationRequested: %v", err)
		return nil
	}

	if p.Title == "" || p.Message == "" {
		logx.WithContext(ctx).Errorf("broadcast %s: empty title or message", p.BroadcastID)
		return nil
	}
	if !notifications.IsValid(p.Type) {
		logx.WithContext(ctx).Errorf("broadcast %s: invalid type %q", p.BroadcastID, p.Type)
		return nil
	}
	if len(p.UserIDs) == 0 {
		return nil
	}

	userIDs := make([]uuid.UUID, 0, len(p.UserIDs))
	for _, idStr := range p.UserIDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			logx.WithContext(ctx).Errorf("broadcast %s: invalid userId %q: %v", p.BroadcastID, idStr, err)
			continue
		}
		userIDs = append(userIDs, id)
	}
	if len(userIDs) == 0 {
		return nil
	}

	inserted, err := repo.Notifications.CreateNotificationsForUsers(ctx, p.Title, p.Message, p.Type, userIDs)
	if err != nil {
		return fmt.Errorf("batch insert broadcast %s chunk %d/%d: %w", p.BroadcastID, p.ChunkIndex, p.ChunkTotal, err)
	}

	logx.WithContext(ctx).Infof("broadcast %s chunk %d/%d: inserted %d/%d notifications",
		p.BroadcastID, p.ChunkIndex, p.ChunkTotal, inserted, len(userIDs))
	return nil
}
