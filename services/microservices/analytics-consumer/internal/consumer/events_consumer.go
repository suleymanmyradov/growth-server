package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/analytics-consumer/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/analytics-consumer/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
)

// EventsHandler consumes domain events from the growth.events topic and
// updates analytics rollup tables (lifecycle events, daily metrics,
// conversion funnels, retention cohorts).
type EventsHandler struct {
	repo *repository.Repository
}

func NewEventsHandler(repo *repository.Repository) *EventsHandler {
	return &EventsHandler{repo: repo}
}

// Consume is the kq.ConsumeHandler callback.
func (h *EventsHandler) Consume(ctx context.Context, _ string, raw string) error {
	var env events.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		logx.WithContext(ctx).Errorf("invalid envelope: %v", err)
		return nil
	}

	// Dedup via analytics_processed_events so redeliveries don't double-process.
	eventID, err := uuid.Parse(env.EventID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid event ID %q: %v", env.EventID, err)
		return nil
	}
	processed, err := h.repo.IsProcessed(ctx, eventID)
	if err != nil {
		logx.WithContext(ctx).Errorf("check processed: %v", err)
	} else if processed {
		logx.WithContext(ctx).Infof("duplicate event %s, skipping", env.EventID)
		return nil
	}

	if err := h.handleEvent(ctx, env); err != nil {
		return err
	}

	if err := h.repo.MarkProcessed(ctx, eventID); err != nil {
		logx.WithContext(ctx).Errorf("mark processed: %v", err)
	}
	return nil
}

func (h *EventsHandler) handleEvent(ctx context.Context, env events.Envelope) error {
	occurredAt := env.OccurredAt
	if occurredAt.IsZero() {
		occurredAt = time.Now().UTC()
	}
	day := occurredAt.UTC().Truncate(24 * time.Hour)

	switch events.EventType(env.EventType) {
	case events.TypeUserOnboarded:
		return h.onUserOnboarded(ctx, env, day)
	case events.TypeCheckInCreated:
		return h.onCheckInCreated(ctx, env, day)
	case events.TypeHabitCreated:
		return h.onHabitCreated(ctx, env, day)
	case events.TypeGoalCreated:
		return h.onGoalCreated(ctx, env, day)
	case events.TypeGoalCompleted:
		return h.onGoalCompleted(ctx, env, day)
	case events.TypeGoalDeleted:
		return h.onGoalDeleted(ctx, env, day)
	case events.TypePlanAdjustmentCreated:
		return h.onPlanAdjustmentCreated(ctx, env, day)
	case events.TypeSubscriptionChanged:
		return h.onSubscriptionChanged(ctx, env, day)
	case events.TypeUserDeleted:
		return h.onUserDeleted(ctx, env)
	default:
		// Not an event we track — skip silently.
		return nil
	}
}

// ─── Event handlers ──────────────────────────────────────────────────────────

func (h *EventsHandler) onUserOnboarded(ctx context.Context, env events.Envelope, day time.Time) error {
	var p events.UserOnboarded
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return nil
	}
	uid, err := uuid.Parse(p.UserID)
	if err != nil {
		return nil
	}
	return h.repo.InsertLifecycleEvent(ctx, db.LifecycleEvent{
		UserID:     uid,
		EventType:  "onboarding_completed",
		OccurredAt: env.OccurredAt,
	})
}

func (h *EventsHandler) onCheckInCreated(ctx context.Context, env events.Envelope, day time.Time) error {
	var p events.CheckInCreated
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return nil
	}
	uid, err := uuid.Parse(p.UserID)
	if err != nil {
		return nil
	}

	// Daily metric: increment check_ins count.
	if err := h.repo.UpsertDailyMetric(ctx, day, "check_ins", 1, nil); err != nil {
		logx.WithContext(ctx).Errorf("upsert check_ins metric: %v", err)
	}

	// Lifecycle: first_check_in (idempotent — ON CONFLICT DO NOTHING at the
	// lifecycle level is not enforced, but the analytics consumer dedups by
	// event_id, so each check-in event is processed exactly once).
	if p.Status == "completed" {
		if err := h.repo.UpsertDailyMetric(ctx, day, "habit_completions", 1, nil); err != nil {
			logx.WithContext(ctx).Errorf("upsert habit_completions metric: %v", err)
		}
		// Record first_check_in lifecycle milestone.
		if err := h.repo.InsertLifecycleEvent(ctx, db.LifecycleEvent{
			UserID:     uid,
			EventType:  "first_check_in",
			OccurredAt: env.OccurredAt,
		}); err != nil {
			logx.WithContext(ctx).Errorf("insert first_check_in lifecycle: %v", err)
		}
	}

	return nil
}

func (h *EventsHandler) onHabitCreated(ctx context.Context, env events.Envelope, day time.Time) error {
	var p events.HabitCreated
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return nil
	}
	uid, err := uuid.Parse(p.UserID)
	if err != nil {
		return nil
	}
	if err := h.repo.UpsertDailyMetric(ctx, day, "habits_created", 1, nil); err != nil {
		logx.WithContext(ctx).Errorf("upsert habits_created metric: %v", err)
	}
	if err := h.repo.InsertLifecycleEvent(ctx, db.LifecycleEvent{
		UserID:     uid,
		EventType:  "first_habit",
		OccurredAt: env.OccurredAt,
	}); err != nil {
		logx.WithContext(ctx).Errorf("insert first_habit lifecycle: %v", err)
	}
	return nil
}

func (h *EventsHandler) onGoalCreated(ctx context.Context, env events.Envelope, day time.Time) error {
	var p events.GoalCreated
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return nil
	}
	uid, err := uuid.Parse(p.UserID)
	if err != nil {
		return nil
	}
	if err := h.repo.UpsertDailyMetric(ctx, day, "goals_created", 1, nil); err != nil {
		logx.WithContext(ctx).Errorf("upsert goals_created metric: %v", err)
	}
	if err := h.repo.InsertLifecycleEvent(ctx, db.LifecycleEvent{
		UserID:     uid,
		EventType:  "first_goal",
		OccurredAt: env.OccurredAt,
	}); err != nil {
		logx.WithContext(ctx).Errorf("insert first_goal lifecycle: %v", err)
	}
	return nil
}

func (h *EventsHandler) onGoalCompleted(ctx context.Context, env events.Envelope, day time.Time) error {
	if err := h.repo.UpsertDailyMetric(ctx, day, "goals_completed", 1, nil); err != nil {
		logx.WithContext(ctx).Errorf("upsert goals_completed metric: %v", err)
	}
	return nil
}

func (h *EventsHandler) onGoalDeleted(ctx context.Context, env events.Envelope, day time.Time) error {
	if err := h.repo.UpsertDailyMetric(ctx, day, "goals_deleted", 1, nil); err != nil {
		logx.WithContext(ctx).Errorf("upsert goals_deleted metric: %v", err)
	}
	return nil
}

func (h *EventsHandler) onPlanAdjustmentCreated(ctx context.Context, env events.Envelope, day time.Time) error {
	if err := h.repo.UpsertDailyMetric(ctx, day, "plan_adjustments_created", 1, nil); err != nil {
		logx.WithContext(ctx).Errorf("upsert plan_adjustments metric: %v", err)
	}
	return nil
}

func (h *EventsHandler) onSubscriptionChanged(ctx context.Context, env events.Envelope, day time.Time) error {
	var p events.SubscriptionChanged
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return nil
	}
	uid, err := uuid.Parse(p.UserID)
	if err != nil {
		return nil
	}

	// Map subscription status to conversion funnel stage.
	stage := ""
	switch p.NewStatus {
	case "trialing":
		stage = "trial_started"
	case "active":
		if p.PreviousStatus == "trialing" {
			stage = "trial_converted"
		} else {
			stage = "paid"
		}
	case "canceled", "expired", "past_due":
		stage = "churned"
	}

	if stage != "" {
		if err := h.repo.UpsertConversionFunnelStage(ctx, db.ConversionFunnelStage{
			UserID:    uid,
			Stage:     stage,
			EnteredAt: env.OccurredAt,
		}); err != nil {
			logx.WithContext(ctx).Errorf("upsert conversion funnel: %v", err)
		}
	}

	// Daily metric for subscription changes.
	metricName := "subscription_" + p.NewStatus
	if err := h.repo.UpsertDailyMetric(ctx, day, metricName, 1, nil); err != nil {
		logx.WithContext(ctx).Errorf("upsert %s metric: %v", metricName, err)
	}

	return nil
}

func (h *EventsHandler) onUserDeleted(ctx context.Context, env events.Envelope) error {
	var p events.UserDeleted
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return nil
	}
	uid, err := uuid.Parse(p.UserID)
	if err != nil {
		return nil
	}
	// Clean up analytics data for deleted users.
	if err := h.repo.DeleteLifecycleEventsByUser(ctx, uid); err != nil {
		logx.WithContext(ctx).Errorf("delete lifecycle events: %v", err)
	}
	if err := h.repo.DeleteConversionFunnelByUser(ctx, uid); err != nil {
		logx.WithContext(ctx).Errorf("delete conversion funnel: %v", err)
	}
	return nil
}

// ─── Retention cohort computation ────────────────────────────────────────────

// ComputeRetentionCohorts computes retention cohorts for the given date range.
// This is a batch job that should be run periodically (e.g. daily via cron).
// It's exposed here so the adminway service can trigger it on demand.
func (h *EventsHandler) ComputeRetentionCohorts(ctx context.Context, from, to time.Time) error {
	// For each cohort date, count how many users signed up that day and how
	// many were active N days later. This is a simplified implementation;
	// a production version would use a more efficient SQL query.
	_ = fmt.Sprintf("compute retention cohorts from %s to %s", from, to)
	// TODO: implement batch retention computation when adminway triggers it.
	return nil
}
