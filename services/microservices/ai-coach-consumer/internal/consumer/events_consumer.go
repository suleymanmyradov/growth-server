package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/prompts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

// AIClient abstracts the AI generation call for testability.
type AIClient interface {
	Generate(ctx context.Context, req ai.GenerateRequest) (ai.GenerateResponse, error)
}

// SafetyClassifier pre-screens user input for safety concerns.
type SafetyClassifier interface {
	Classify(ctx context.Context, text string) (safety.Verdict, error)
}

// Publisher publishes event envelopes.
type Publisher interface {
	Publish(ctx context.Context, env events.Envelope) error
}

// DLQPusher pushes messages to the dead-letter topic.
type DLQPusher interface {
	Publish(ctx context.Context, msg events.DLQMessage) error
}

// TxRunner executes work inside a PostgreSQL transaction.
type TxRunner interface {
	Run(ctx context.Context, userID string, fn func(pgx.Tx) error) error
}

// EventsHandler consumes domain events from the growth.events topic and
// generates AI coaching feedback for check-in events.
type EventsHandler struct {
	repo        *repository.Repository
	txRunner    TxRunner
	ai          AIClient
	pub         Publisher
	dlqPub      DLQPusher
	classifier  SafetyClassifier
	sem         chan struct{}
	aiTimeout   time.Duration
	serviceName string
	safetyCache sync.Map // habitName -> safety.Verdict
}

// EventsHandlerOptions carries optional configuration for the handler.
type EventsHandlerOptions struct {
	TxRunner    TxRunner
	DLQPub      DLQPusher
	AITimeout   time.Duration
	Concurrency int
	ServiceName string
}

// NewEventsHandler creates a handler with the given dependencies.
func NewEventsHandler(repo *repository.Repository, aiClient AIClient, pub Publisher, classifier SafetyClassifier, opts *EventsHandlerOptions) *EventsHandler {
	h := &EventsHandler{
		repo:        repo,
		ai:          aiClient,
		pub:         pub,
		classifier:  classifier,
		aiTimeout:   30 * time.Second,
		serviceName: "ai-coach-consumer",
	}
	if opts != nil {
		if opts.TxRunner != nil {
			h.txRunner = opts.TxRunner
		}
		h.dlqPub = opts.DLQPub
		if opts.AITimeout > 0 {
			h.aiTimeout = opts.AITimeout
		}
		if opts.Concurrency > 0 {
			h.sem = make(chan struct{}, opts.Concurrency)
		}
		if opts.ServiceName != "" {
			h.serviceName = opts.ServiceName
		}
	}
	return h
}

// Consume is the kq.ConsumeHandler callback.
func (h *EventsHandler) Consume(ctx context.Context, _ string, raw string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "EventsHandler.Consume")
	defer span.End()

	// Back-pressure: acquire a slot or respect cancellation.
	if h.sem != nil {
		select {
		case h.sem <- struct{}{}:
			defer func() { <-h.sem }()
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	timer := prometheus.NewTimer(eventsProcessingDuration.WithLabelValues("unknown"))
	defer timer.ObserveDuration()

	var env events.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		logx.WithContext(ctx).Errorf("invalid envelope JSON, sending to DLQ: %v", err)
		h.sendToDLQ(ctx, events.Envelope{}, raw, "invalid_envelope", true)
		eventsConsumedTotal.WithLabelValues("unknown", "dlq").Inc()
		return nil // permanent error — commit offset
	}

	timer.ObserveDuration()
	timer = prometheus.NewTimer(eventsProcessingDuration.WithLabelValues(env.EventType))

	logx.WithContext(ctx).Infof("received event: type=%s eventID=%s version=%d", env.EventType, env.EventID, env.Version)

	// Validate envelope contract.
	if err := env.Validate(); err != nil {
		logx.WithContext(ctx).Errorf("envelope validation failed, sending to DLQ: type=%s eventID=%s err=%v", env.EventType, env.EventID, err)
		h.sendToDLQ(ctx, env, raw, fmt.Sprintf("validation_failed: %v", err), true)
		eventsConsumedTotal.WithLabelValues(env.EventType, "dlq").Inc()
		return nil
	}

	eventID, err := uuid.Parse(env.EventID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid event ID %q, sending to DLQ: type=%s", env.EventID, env.EventType)
		h.sendToDLQ(ctx, env, raw, "invalid_event_id", true)
		eventsConsumedTotal.WithLabelValues(env.EventType, "dlq").Inc()
		return nil
	}

	// Idempotency check.
	processed, err := h.repo.IsProcessed(ctx, eventID)
	if err != nil {
		logx.WithContext(ctx).Errorf("idempotency check failed, will retry: type=%s eventID=%s err=%v", env.EventType, env.EventID, err)
		return fmt.Errorf("idempotency check: %w", err) // transient — will retry
	}
	if processed {
		logx.WithContext(ctx).Infof("duplicate event skipped: type=%s eventID=%s", env.EventType, env.EventID)
		eventsDuplicateTotal.WithLabelValues(env.EventType).Inc()
		eventsConsumedTotal.WithLabelValues(env.EventType, "duplicate").Inc()
		return nil
	}

	if events.EventType(env.EventType) != events.TypeCheckInCreated {
		logx.WithContext(ctx).Infof("ignoring non-check-in event: type=%s eventID=%s", env.EventType, env.EventID)
		eventsConsumedTotal.WithLabelValues(env.EventType, "ignored").Inc()
		return nil
	}

	logx.WithContext(ctx).Infof("processing check-in event: eventID=%s", env.EventID)
	eventsConsumedTotal.WithLabelValues(env.EventType, "processing").Inc()
	err = h.onCheckInCreated(ctx, env, eventID)
	if err != nil {
		if IsTransientError(err) {
			logx.WithContext(ctx).Errorf("transient error processing event, will retry: eventID=%s err=%v", env.EventID, err)
			eventsConsumedTotal.WithLabelValues(env.EventType, "retry").Inc()
			return err
		}
		logx.WithContext(ctx).Errorf("permanent error processing event, sending to DLQ: eventID=%s err=%v", env.EventID, err)
		h.sendToDLQ(ctx, env, raw, err.Error(), true)
		eventsConsumedTotal.WithLabelValues(env.EventType, "dlq").Inc()
		return nil
	}

	logx.WithContext(ctx).Infof("event processed successfully: type=%s eventID=%s", env.EventType, env.EventID)
	eventsConsumedTotal.WithLabelValues(env.EventType, "success").Inc()
	return nil
}

func (h *EventsHandler) onCheckInCreated(ctx context.Context, env events.Envelope, eventID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "EventsHandler.onCheckInCreated")
	defer span.End()

	if err := ctx.Err(); err != nil {
		return err
	}

	var p events.CheckInCreated
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.WithContext(ctx).Errorf("failed to unmarshal CheckInCreated payload: eventID=%s err=%v", env.EventID, err)
		return fmt.Errorf("unmarshal CheckInCreated: %w", err) // permanent
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid userID %q: eventID=%s", p.UserID, env.EventID)
		return fmt.Errorf("invalid userID %q: %w", p.UserID, err) // permanent
	}

	habitID, err := uuid.Parse(p.HabitID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid habitID %q: eventID=%s user=%s", p.HabitID, env.EventID, p.UserID)
		return fmt.Errorf("invalid habitID %q: %w", p.HabitID, err) // permanent
	}

	checkInID, err := uuid.Parse(p.CheckInID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid checkInID %q: eventID=%s user=%s", p.CheckInID, env.EventID, p.UserID)
		return fmt.Errorf("invalid checkInID %q: %w", p.CheckInID, err) // permanent
	}

	logx.WithContext(ctx).Infof("check-in created event: user=%s habit=%s checkIn=%s status=%s streak=%d",
		p.UserID, p.HabitName, p.CheckInID, p.Status, p.Streak)

	// Look up accountability style (default to "balanced" on error).
	accountabilityStyle := "balanced"
	if style, err := h.repo.GetAccountabilityStyle(ctx, userID); err == nil && style != "" {
		accountabilityStyle = style
	} else if err != nil {
		logx.WithContext(ctx).Infof("failed to get accountability style, using default: user=%s err=%v", p.UserID, err)
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	// Compute recent 7-day pattern.
	recentPattern := h.buildRecentPattern(ctx, userID, habitID)

	// Build prompts.
	promptInput := prompts.CheckInFeedbackInput{
		HabitName:           p.HabitName,
		Status:              p.Status,
		AccountabilityStyle: accountabilityStyle,
		Streak:              p.Streak,
		RecentPattern:       recentPattern,
	}

	// Safety check on user-generated fields before sending to the model.
	if h.classifier != nil {
		var verdict safety.Verdict
		if cached, ok := h.safetyCache.Load(p.HabitName); ok {
			verdict = cached.(safety.Verdict)
		} else {
			var err error
			verdict, err = h.classifier.Classify(ctx, p.HabitName)
			if err != nil {
				logx.WithContext(ctx).Errorf("safety classification error, will retry: user=%s habit=%s err=%v", p.UserID, p.HabitName, err)
				// Treat safety-classifier errors as transient so we retry rather than
				// skip a potentially-valid event.
				return fmt.Errorf("safety classification: %w", err)
			}
			h.safetyCache.Store(p.HabitName, verdict)
		}
		if verdict.Category != safety.CategorySafe {
			logx.WithContext(ctx).Infof("safety block: user=%s habit=%s category=%s confidence=%.2f reason=%s",
				p.UserID, p.HabitName, verdict.Category, verdict.Confidence, verdict.Reason)
			aiSafetyBlockedTotal.Inc()
			// Safety block is permanent — mark processed so we don't retry.
			_ = h.repo.MarkProcessed(ctx, eventID)
			return nil
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	system := prompts.BuildSystemPrompt(accountabilityStyle)
	user := prompts.BuildUserPrompt(promptInput)

	// Call AI with a bounded timeout so a slow/hung call does not block
	// graceful shutdown or starve other messages.
	aiCtx, cancel := context.WithTimeout(ctx, h.aiTimeout)
	defer cancel()

	logx.WithContext(ctx).Infof("calling AI for check-in feedback: user=%s habit=%s timeout=%s", p.UserID, p.HabitName, h.aiTimeout)

	aiStart := time.Now()
	resp, err := h.ai.Generate(aiCtx, ai.GenerateRequest{
		ModelProfile: ai.ModelCheap,
		System:       system,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: user},
		},
		Metadata: ai.Metadata{
			UserID:  p.UserID,
			Feature: "check-in-feedback",
		},
	})
	aiDuration := time.Since(aiStart).Seconds()
	if err != nil {
		aiGenerationDuration.WithLabelValues("error").Observe(aiDuration)
		logx.WithContext(ctx).Errorf("AI feedback generation failed: user=%s habit=%s duration=%.2fs err=%v", p.UserID, p.HabitName, aiDuration, err)
		// AI errors from the client are already retried internally.
		// If they surface here they are either transient (rate limit after retries)
		// or permanent (bad request). We conservatively treat them as transient
		// unless the context was cancelled by us.
		if aiCtx.Err() != nil && ctx.Err() == nil {
			// Our timeout fired but the parent context is still alive.
			return fmt.Errorf("ai generation timeout: %w", err)
		}
		return fmt.Errorf("ai generation failed: %w", err)
	}
	aiGenerationDuration.WithLabelValues("ok").Observe(aiDuration)
	logx.WithContext(ctx).Infof("AI feedback generated: user=%s habit=%s model=%s duration=%.2fs tokens=%d",
		p.UserID, p.HabitName, resp.ModelID, aiDuration, resp.Usage.TotalTokens)

	if err := ctx.Err(); err != nil {
		return err
	}

	content := resp.Message.Content
	feedbackID := uuid.New()

	// Persist feedback and mark event processed atomically inside a transaction.
	if h.txRunner != nil {
		err = h.txRunner.Run(ctx, p.UserID, func(tx pgx.Tx) error {
			txRepo := h.repo.WithTx(tx)
			if err := txRepo.InsertAIFeedback(ctx, db.InsertAIFeedbackParams{
				ID:        feedbackID,
				UserID:    userID,
				CheckInID: checkInID,
				HabitID:   habitID,
				Content:   content,
				Model:     resp.ModelID,
			}); err != nil {
				return fmt.Errorf("insert ai_feedback: %w", err)
			}
			if err := txRepo.MarkProcessed(ctx, eventID); err != nil {
				return fmt.Errorf("mark processed: %w", err)
			}
			return nil
		})
		if err != nil {
			logx.WithContext(ctx).Errorf("transaction failed, will retry: user=%s eventID=%s err=%v", p.UserID, env.EventID, err)
			return fmt.Errorf("transaction failed: %w", err) // transient — will retry
		}
	} else {
		// Fallback for tests or local dev without a runner.
		if err := h.repo.InsertAIFeedback(ctx, db.InsertAIFeedbackParams{
			ID:        feedbackID,
			UserID:    userID,
			CheckInID: checkInID,
			HabitID:   habitID,
			Content:   content,
			Model:     resp.ModelID,
		}); err != nil {
			logx.WithContext(ctx).Errorf("insert ai_feedback failed: user=%s eventID=%s err=%v", p.UserID, env.EventID, err)
			return fmt.Errorf("insert ai_feedback: %w", err)
		}
		if err := h.repo.MarkProcessed(ctx, eventID); err != nil {
			logx.WithContext(ctx).Errorf("mark processed failed: user=%s eventID=%s err=%v", p.UserID, env.EventID, err)
			return fmt.Errorf("mark processed: %w", err)
		}
	}

	logx.WithContext(ctx).Infof("feedback persisted: user=%s habit=%s feedbackID=%s", p.UserID, p.HabitName, feedbackID)

	// Publish feedback generated event.
	if h.pub != nil {
		feedbackEnv, err := events.NewEnvelope(events.TypeCheckInFeedbackGenerated, events.CheckInFeedbackGenerated{
			UserID:    p.UserID,
			CheckInID: checkInID.String(),
			HabitID:   p.HabitID,
			Content:   content,
		})
		if err != nil {
			logx.WithContext(ctx).Errorf("build feedback envelope: user=%s err=%v", p.UserID, err)
		} else if err := h.pub.Publish(ctx, feedbackEnv); err != nil {
			logx.WithContext(ctx).Errorf("publish feedback event: user=%s err=%v", p.UserID, err)
		} else {
			logx.WithContext(ctx).Infof("published feedback event: user=%s habit=%s", p.UserID, p.HabitName)
		}
	}

	logx.WithContext(ctx).Infof("generated AI feedback for user %s habit %s", p.UserID, p.HabitID)
	return nil
}

func (h *EventsHandler) buildRecentPattern(ctx context.Context, userID, habitID uuid.UUID) string {
	if err := ctx.Err(); err != nil {
		return ""
	}
	now := time.Now().UTC()
	start := now.AddDate(0, 0, -7)
	checkIns, err := h.repo.GetCheckInsForWeek(ctx, userID, start, now)
	if err != nil {
		logx.WithContext(ctx).Infof("failed to get check-ins for pattern, returning empty: user=%s err=%v", userID, err)
		return ""
	}

	var habitChecks int
	var completed int
	for _, c := range checkIns {
		if c.HabitID == habitID {
			habitChecks++
			if c.Status == "completed" {
				completed++
			}
		}
	}
	if habitChecks == 0 {
		return ""
	}
	pattern := fmt.Sprintf("completed %d of last %d check-ins", completed, habitChecks)
	logx.WithContext(ctx).Debugf("recent pattern: user=%s habit=%s pattern=%s", userID, habitID, pattern)
	return pattern
}

func (h *EventsHandler) sendToDLQ(ctx context.Context, env events.Envelope, raw, reason string, permanent bool) {
	if h.dlqPub == nil {
		logx.WithContext(ctx).Infof("no DLQ publisher configured, dropping message: eventID=%s reason=%s", env.EventID, reason)
		return
	}
	msg := events.DLQMessage{
		Original:    env,
		Raw:         raw,
		Reason:      reason,
		Permanent:   permanent,
		ServiceName: h.serviceName,
		OccurredAt:  time.Now().UTC(),
	}
	if err := h.dlqPub.Publish(ctx, msg); err != nil {
		logx.WithContext(ctx).Errorf("failed to publish to DLQ: eventID=%s reason=%s err=%v", env.EventID, reason, err)
	} else {
		logx.WithContext(ctx).Infof("message sent to DLQ: eventID=%s type=%s reason=%s permanent=%v", env.EventID, env.EventType, reason, permanent)
		eventsDLQTotal.WithLabelValues(env.EventType, reason).Inc()
	}
}
