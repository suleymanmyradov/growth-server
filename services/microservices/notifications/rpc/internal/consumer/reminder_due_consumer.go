package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/postgres"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/delivery"
	internalnotification "github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/notification"
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

// EventsPublisher publishes domain events to the growth.events topic. Used by
// the coach_digest handler to publish CoachDigestRequested so the
// ai-coach-consumer can generate the digest asynchronously. Nil means digest
// reminders are logged and dropped (dev/test without Kafka).
type EventsPublisher interface {
	Publish(ctx context.Context, env events.Envelope) error
}

// ReminderDueHandler consumes events from the growth.reminder.due topic and
// materializes notification rows, then enqueues follow-up reminders.
type ReminderDueHandler struct {
	repo       *repository.Repository
	clock      Clock
	txRunner   *postgres.PgxTxRunner
	dlq        DLQPublisher
	pushSender PushSender
	eventsPub  EventsPublisher
}

// NewReminderDueHandler creates a handler with the given dependencies. If
// txRunner is nil, the handler+mark pair runs without a transaction. If dlq
// is nil, poison messages are logged and dropped instead of being routed to
// a dead-letter topic. If pushSender is nil, no push is delivered. If
// eventsPub is nil, coach_digest reminders are logged and dropped.
func NewReminderDueHandler(repo *repository.Repository, clock Clock, txRunner *postgres.PgxTxRunner, dlq DLQPublisher, pushSender PushSender, eventsPub EventsPublisher) *ReminderDueHandler {
	if clock == nil {
		clock = realClock{}
	}
	return &ReminderDueHandler{repo: repo, clock: clock, txRunner: txRunner, dlq: dlq, pushSender: pushSender, eventsPub: eventsPub}
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
	case "coach_digest":
		return h.onCoachDigest(ctx, repo, userID, p)
	case "streak_warning_scan":
		return h.onStreakWarning(ctx, repo, userID, p)
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

func (h *ReminderDueHandler) createNotification(ctx context.Context, repo *repository.Repository, userID uuid.UUID, itemType, title, message string, dest delivery.Destination, resourceID uuid.UUID, deduplicationKey string, metadata map[string]any, email bool) (uuid.UUID, error) {
	pref, err := repo.Preferences.Get(ctx, userID)
	if err != nil {
		return uuid.Nil, fmt.Errorf("get notification preferences: %w", err)
	}
	created, err := internalnotification.Create(ctx, repo, internalnotification.Request{
		UserID:           userID,
		Type:             itemType,
		Title:            title,
		Message:          message,
		Destination:      string(dest),
		ResourceID:       resourceID,
		DeduplicationKey: deduplicationKey,
		Metadata:         metadata,
		Push:             pref.PushNotifications,
		Email:            email && pref.EmailNotifications,
	})
	if err != nil {
		return uuid.Nil, err
	}
	return created.ID, nil
}

func (h *ReminderDueHandler) onHabitReminder(ctx context.Context, repo *repository.Repository, userID uuid.UUID, _ events.ReminderDue) error {
	rs, err := repo.ReminderState.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get reminder state: %w", err)
	}

	if rs.ActiveHabitCount == 0 || !rs.HabitReminders {
		return nil
	}

	message := fmt.Sprintf("You have %d habits to check in on today", rs.ActiveHabitCount)
	if _, err := h.createNotification(ctx, repo, userID, "habit_reminder", "Time to check in", message, delivery.DestinationActivity, uuid.Nil, "", nil, false); err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

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

	message := "You missed your check-in today. Don't worry, tomorrow is a fresh start!"
	if _, err := h.createNotification(ctx, repo, userID, "missed_check_in", "Missed check-in", message, delivery.DestinationActivity, uuid.Nil, "", nil, false); err != nil {
		return fmt.Errorf("create notification: %w", err)
	}

	return nil
}

func (h *ReminderDueHandler) onWeeklyReview(ctx context.Context, repo *repository.Repository, userID uuid.UUID, _ events.ReminderDue) error {
	pref, err := repo.Preferences.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get notification preferences: %w", err)
	}
	rs, err := repo.ReminderState.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get reminder state for weekly reschedule: %w", err)
	}
	if pref.SundayReview {
		localDate := h.clock.Now().In(timezoneOrUTC(rs.Timezone)).Format("2006-01-02")
		deduplicationKey := fmt.Sprintf("weekly-review:%s:%s", userID, localDate)
		if _, err := h.createNotification(ctx, repo, userID, "weekly_review", "Your weekly review is ready", "Take a few minutes to reflect on your week and plan what comes next.", delivery.DestinationWeeklyReview, uuid.Nil, deduplicationKey, nil, true); err != nil {
			return fmt.Errorf("create notification: %w", err)
		}
	}

	nextSun, err := scheduler.NextWeekday(h.clock.Now(), rs.Timezone, time.Sunday, 18, 0)
	if err != nil {
		return fmt.Errorf("compute next weekly review: %w", err)
	}
	if pref.SundayReview {
		if _, err := repo.Reminders.Enqueue(ctx, userID, "weekly_review", nextSun, nil); err != nil {
			return fmt.Errorf("enqueue weekly_review: %w", err)
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

	if _, err := h.createNotification(ctx, repo, userID, "encouragement", title, msg, delivery.DestinationHabitDetail, uuid.Nil, "", meta, false); err != nil {
		return fmt.Errorf("create notification: %w", err)
	}
	return nil
}

func (h *ReminderDueHandler) onStreakWarning(ctx context.Context, repo *repository.Repository, userID uuid.UUID, _ events.ReminderDue) error {
	pref, err := repo.Preferences.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get notification preferences: %w", err)
	}
	rs, err := repo.ReminderState.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get reminder state: %w", err)
	}
	localNow := h.clock.Now().In(timezoneOrUTC(rs.Timezone))
	if pref.StreakWarnings {
		habits, listErr := repo.HabitState.ListAtRisk(ctx, userID, localNow, 3)
		if listErr != nil {
			return fmt.Errorf("list habits at streak risk: %w", listErr)
		}
		if len(habits) > 0 {
			names := make([]string, 0, len(habits))
			habitIDs := make([]string, 0, len(habits))
			for _, habit := range habits {
				names = append(names, habit.HabitName)
				habitIDs = append(habitIDs, habit.HabitID.String())
			}
			message := fmt.Sprintf("Check in on %s before today ends to keep your momentum.", strings.Join(names, ", "))
			deduplicationKey := fmt.Sprintf("streak-warning:%s:%s", userID, localNow.Format("2006-01-02"))
			if _, createErr := h.createNotification(ctx, repo, userID, "streak_warning", "Your streak is at risk", message, delivery.DestinationActivity, uuid.Nil, deduplicationKey, map[string]any{"habitIds": habitIDs}, true); createErr != nil {
				return fmt.Errorf("create streak warning: %w", createErr)
			}
		}
		next, nextErr := scheduler.NextDailyAt(h.clock.Now(), rs.Timezone, 20, 0)
		if nextErr != nil {
			return fmt.Errorf("compute next streak warning: %w", nextErr)
		}
		if _, enqueueErr := repo.Reminders.Enqueue(ctx, userID, "streak_warning_scan", next, nil); enqueueErr != nil {
			return fmt.Errorf("enqueue next streak warning: %w", enqueueErr)
		}
	}
	return nil
}

// onCoachDigest handles the daily coach digest reminder. Instead of creating
// a notification directly, it publishes a CoachDigestRequested event to the
// growth.events topic. The ai-coach-consumer picks it up, fetches all of the
// user's check-ins for the day, and generates one combined AI feedback
// message — which then flows back as a CheckInFeedbackGenerated event that
// the notifications consumer turns into a single notification.
//
// If the user didn't check in today, the digest is skipped (the
// missed_check_in reminder handles that case). After firing, the next day's
// digest is enqueued.
func (h *ReminderDueHandler) onCoachDigest(ctx context.Context, repo *repository.Repository, userID uuid.UUID, _ events.ReminderDue) error {
	rs, err := repo.ReminderState.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get reminder state for coach_digest: %w", err)
	}

	// Skip if the user didn't check in today — missed_check_in handles that.
	if rs.CheckedInCountToday == 0 {
		logx.WithContext(ctx).Infof("coach_digest skipped: user %s has no check-ins today", userID)
		return h.enqueueNextCoachDigest(ctx, repo, userID, rs)
	}

	// Compute today's date in the user's timezone so the consumer fetches the
	// correct day's check-ins.
	tz := rs.Timezone
	if tz == "" {
		tz = "UTC"
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		loc = time.UTC
	}
	today := h.clock.Now().In(loc).Format("2006-01-02")

	// Publish CoachDigestRequested so ai-coach-consumer generates the digest.
	if h.eventsPub != nil {
		env, err := events.NewEnvelope(events.TypeCoachDigestRequested, events.CoachDigestRequested{
			UserID: userID.String(),
			Date:   today,
		})
		if err != nil {
			return fmt.Errorf("build coach_digest envelope: %w", err)
		}
		if err := h.eventsPub.Publish(ctx, env); err != nil {
			return fmt.Errorf("publish coach_digest event: %w", err)
		}
		logx.WithContext(ctx).Infof("published coach_digest request: user=%s date=%s", userID, today)
	} else {
		logx.WithContext(ctx).Infof("coach_digest reminder fired but no events publisher configured: user=%s", userID)
	}

	return h.enqueueNextCoachDigest(ctx, repo, userID, rs)
}

// enqueueNextCoachDigest schedules the next day's coach_digest reminder.
func (h *ReminderDueHandler) enqueueNextCoachDigest(ctx context.Context, repo *repository.Repository, userID uuid.UUID, rs db.ReminderState) error {
	now := h.clock.Now()
	next, err := scheduler.NextCoachDigest(now, rs.Timezone, rs.CheckInTime)
	if err != nil {
		logx.WithContext(ctx).Errorf("next coach_digest: %v", err)
		return nil
	}
	if _, err := repo.Reminders.Enqueue(ctx, userID, "coach_digest", next, nil); err != nil {
		return fmt.Errorf("enqueue next coach_digest: %w", err)
	}
	return nil
}
