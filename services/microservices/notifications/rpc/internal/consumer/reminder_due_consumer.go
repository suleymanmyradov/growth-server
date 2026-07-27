package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/delivery"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/scheduler"
	"github.com/zeromicro/go-zero/core/logx"
)

// PushSender is the interface the reminder consumer uses to deliver push
// notifications. The delivery.Sender implements this; nil means no push.
type PushSender interface {
	Send(ctx context.Context, userID uuid.UUID, payload delivery.Payload) (int, error)
}

// ReminderDueHandler consumes events from the growth.reminder.due topic and
// materializes notification rows, then enqueues follow-up reminders.
type ReminderDueHandler struct {
	repo       *repository.Repository
	clock      Clock
	txRunner   *postgres.PgxTxRunner
	dlq        DLQPublisher
	pushSender PushSender
}

// NewReminderDueHandler creates a handler with the given dependencies. If
// txRunner is nil, the handler+mark pair runs without a transaction. If dlq
// is nil, poison messages are logged and dropped instead of being routed to
// a dead-letter topic. If pushSender is nil, no push is delivered.
func NewReminderDueHandler(repo *repository.Repository, clock Clock, txRunner *postgres.PgxTxRunner, dlq DLQPublisher, pushSender PushSender) *ReminderDueHandler {
	if clock == nil {
		clock = realClock{}
	}
	return &ReminderDueHandler{repo: repo, clock: clock, txRunner: txRunner, dlq: dlq, pushSender: pushSender}
}

// sendToDLQ publishes a poison message to the DLQ. If no DLQ publisher is
// configured, it logs the rejection instead.
func (h *ReminderDueHandler) sendToDLQ(ctx context.Context, env events.Envelope, raw, reason string) {
	msg := events.DLQMessage{
		Original:    env,
		Raw:         raw,
		Reason:      reason,
		Permanent:   true,
		ServiceName: "notifications.reminder-due",
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

// Consume is the kq.ConsumeHandler callback for the reminder.due topic.
//
// When a txRunner is configured, the handler dispatch and the processed_events
// marking run inside a single transaction so that a crash or Mark failure
// cannot leave a notification created without a processed_events row — which
// would cause duplicate notifications on redelivery.
func (h *ReminderDueHandler) Consume(ctx context.Context, _ string, raw string) error {
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
		return nil
	}

	var p events.ReminderDue
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		h.sendToDLQ(ctx, env, raw, fmt.Sprintf("unmarshal ReminderDue: %v", err))
		return nil
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		h.sendToDLQ(ctx, env, raw, fmt.Sprintf("invalid userID %q: %v", p.UserID, err))
		return nil
	}

	if h.txRunner != nil && h.repo.ProcessedEvents != nil {
		return h.txRunner.Run(ctx, "", func(tx pgx.Tx) error {
			txRepo := repository.NewRepositoryFromTx(tx)

			if err := h.dispatch(ctx, txRepo, userID, p); err != nil {
				return err
			}
			if err := txRepo.ProcessedEvents.Mark(ctx, eventID); err != nil {
				return fmt.Errorf("mark event %s processed: %w", env.EventID, err)
			}
			return nil
		})
	}

	// Non-transactional fallback.
	if err := h.dispatch(ctx, h.repo, userID, p); err != nil {
		return err
	}
	if h.repo.ProcessedEvents != nil {
		if err := h.repo.ProcessedEvents.Mark(ctx, eventID); err != nil {
			logx.WithContext(ctx).Errorf("mark event %s processed: %v", env.EventID, err)
		}
	}
	return nil
}

func (h *ReminderDueHandler) dispatch(ctx context.Context, repo *repository.Repository, userID uuid.UUID, p events.ReminderDue) error {
	switch p.Type {
	case "habit_reminder":
		return h.onHabitReminder(ctx, repo, userID, p)
	case "missed_check_in":
		return h.onMissedCheckIn(ctx, repo, userID, p)
	case "weekly_review":
		return h.onWeeklyReview(ctx, repo, userID, p)
	case "encouragement":
		return h.onEncouragement(ctx, repo, userID, p)
	default:
		logx.WithContext(ctx).Infof("unhandled reminder type %s", p.Type)
		return nil
	}
}

// sendPush delivers a best-effort push notification for a newly created
// in-app notification row. Push failures are logged but never fail the
// reminder processing. When pushSender is nil (e.g. in tests or when Expo is
// disabled), this is a no-op.
func (h *ReminderDueHandler) sendPush(ctx context.Context, userID uuid.UUID, title, body string, notificationID uuid.UUID, dest delivery.Destination, resourceID uuid.UUID) {
	if h.pushSender == nil {
		return
	}
	payload, err := delivery.NewPayload(title, body, notificationID, dest, resourceID)
	if err != nil {
		logx.WithContext(ctx).Errorf("push payload construction failed: %v", err)
		return
	}
	if _, err := h.pushSender.Send(ctx, userID, payload); err != nil {
		logx.WithContext(ctx).Errorf("push delivery failed for notification %s: %v", notificationID, err)
	}
}

func (h *ReminderDueHandler) onHabitReminder(ctx context.Context, repo *repository.Repository, userID uuid.UUID, _ events.ReminderDue) error {
	rs, err := repo.ReminderState.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get reminder state: %w", err)
	}

	if rs.ActiveHabitCount == 0 || !rs.HabitReminders {
		return nil
	}

	notif, err := repo.Notifications.CreateNotification(ctx, "Time to check in", fmt.Sprintf("You have %d habits to check in on today", rs.ActiveHabitCount), "habit_reminder", userID)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	h.sendPush(ctx, userID, "Time to check in", fmt.Sprintf("You have %d habits to check in on today", rs.ActiveHabitCount), notif.ID, delivery.DestinationActivity, uuid.Nil)

	now := h.clock.Now()

	// Enqueue tomorrow's habit_reminder at user's check_in_time.
	next, err := scheduler.NextOccurrence(now, rs.Timezone, rs.CheckInTime)
	if err != nil {
		logx.WithContext(ctx).Errorf("next occurrence: %v", err)
	} else {
		if _, err := repo.Reminders.Enqueue(ctx, userID, "habit_reminder", next, nil); err != nil {
			logx.WithContext(ctx).Errorf("enqueue next habit_reminder: %v", err)
		}
	}

	// Enqueue today's missed_check_in at now + 2h.
	if _, err := repo.Reminders.Enqueue(ctx, userID, "missed_check_in",
		now.Add(2*time.Hour), nil); err != nil {
		logx.WithContext(ctx).Errorf("enqueue missed_check_in: %v", err)
	}

	return nil
}

func (h *ReminderDueHandler) onMissedCheckIn(ctx context.Context, repo *repository.Repository, userID uuid.UUID, _ events.ReminderDue) error {
	rs, err := repo.ReminderState.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get reminder state: %w", err)
	}

	// User checked in today if last_check_in_date is today (in their timezone).
	if isCheckedInToday(rs, h.clock.Now()) {
		return nil
	}

	notif, err := repo.Notifications.CreateNotification(ctx, "Missed check-in", "You missed your check-in today. Don't worry, tomorrow is a fresh start!", "missed_check_in", userID)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	h.sendPush(ctx, userID, "Missed check-in", "You missed your check-in today. Don't worry, tomorrow is a fresh start!", notif.ID, delivery.DestinationActivity, uuid.Nil)

	return nil
}

func (h *ReminderDueHandler) onWeeklyReview(ctx context.Context, repo *repository.Repository, userID uuid.UUID, _ events.ReminderDue) error {
	notif, err := repo.Notifications.CreateNotification(ctx, "Weekly review", "Reflect on your week", "weekly_review", userID)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	h.sendPush(ctx, userID, "Weekly review", "Reflect on your week", notif.ID, delivery.DestinationWeeklyReview, uuid.Nil)

	// Enqueue next Sunday 18:00 local.
	rs, err := repo.ReminderState.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get reminder state for weekly reschedule: %w", err)
	}

	nextSun, err := scheduler.NextWeekday(h.clock.Now(), rs.Timezone, time.Sunday, 18, 0)
	if err != nil {
		logx.WithContext(ctx).Errorf("next weekday: %v", err)
	} else {
		if _, err := repo.Reminders.Enqueue(ctx, userID, "weekly_review", nextSun, nil); err != nil {
			logx.WithContext(ctx).Errorf("enqueue weekly_review: %v", err)
		}
	}

	return nil
}

// isCheckedInToday returns true if the user has checked in today (in their timezone).
func isCheckedInToday(rs db.ReminderState, now time.Time) bool {
	if !rs.LastCheckInDate.Valid || rs.CheckedInCountToday == 0 {
		return false
	}
	today := now.In(timezoneOrUTC(rs.Timezone)).Format("2006-01-02")
	return rs.LastCheckInDate.Time.Format("2006-01-02") == today
}

func timezoneOrUTC(tz string) *time.Location {
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

func (h *ReminderDueHandler) onEncouragement(ctx context.Context, repo *repository.Repository, userID uuid.UUID, p events.ReminderDue) error {
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

	notif, err := repo.Notifications.CreateNotification(ctx, title, msg, "encouragement", userID)
	if err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	h.sendPush(ctx, userID, title, msg, notif.ID, delivery.DestinationHabitDetail, uuid.Nil)

	return nil
}
