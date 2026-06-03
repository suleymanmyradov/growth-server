package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/consumer/internal/prompts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/consumer/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/consumer/internal/repository/db"
	"github.com/zeromicro/go-zero/core/logx"
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
		h.sendToDLQ(ctx, events.Envelope{}, raw, "invalid_envelope", true)
		eventsConsumedTotal.WithLabelValues("unknown", "dlq").Inc()
		return nil // permanent error — commit offset
	}

	timer.ObserveDuration()
	timer = prometheus.NewTimer(eventsProcessingDuration.WithLabelValues(env.EventType))

	// Validate envelope contract.
	if err := env.Validate(); err != nil {
		h.sendToDLQ(ctx, env, raw, fmt.Sprintf("validation_failed: %v", err), true)
		eventsConsumedTotal.WithLabelValues(env.EventType, "dlq").Inc()
		return nil
	}

	eventID, err := uuid.Parse(env.EventID)
	if err != nil {
		h.sendToDLQ(ctx, env, raw, "invalid_event_id", true)
		eventsConsumedTotal.WithLabelValues(env.EventType, "dlq").Inc()
		return nil
	}

	// Idempotency check.
	processed, err := h.repo.IsProcessed(ctx, eventID)
	if err != nil {
		return fmt.Errorf("idempotency check: %w", err) // transient — will retry
	}
	if processed {
		eventsDuplicateTotal.WithLabelValues(env.EventType).Inc()
		eventsConsumedTotal.WithLabelValues(env.EventType, "duplicate").Inc()
		return nil
	}

	if events.EventType(env.EventType) != events.TypeCheckInCreated {
		eventsConsumedTotal.WithLabelValues(env.EventType, "ignored").Inc()
		return nil
	}

	eventsConsumedTotal.WithLabelValues(env.EventType, "processing").Inc()
	err = h.onCheckInCreated(ctx, env, eventID)
	if err != nil {
		if IsTransientError(err) {
			eventsConsumedTotal.WithLabelValues(env.EventType, "retry").Inc()
			return err
		}
		h.sendToDLQ(ctx, env, raw, err.Error(), true)
		eventsConsumedTotal.WithLabelValues(env.EventType, "dlq").Inc()
		return nil
	}

	eventsConsumedTotal.WithLabelValues(env.EventType, "success").Inc()
	return nil
}

func (h *EventsHandler) onCheckInCreated(ctx context.Context, env events.Envelope, eventID uuid.UUID) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	var p events.CheckInCreated
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return fmt.Errorf("unmarshal CheckInCreated: %w", err) // permanent
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return fmt.Errorf("invalid userID %q: %w", p.UserID, err) // permanent
	}

	habitID, err := uuid.Parse(p.HabitID)
	if err != nil {
		return fmt.Errorf("invalid habitID %q: %w", p.HabitID, err) // permanent
	}

	checkInID, err := uuid.Parse(p.CheckInID)
	if err != nil {
		return fmt.Errorf("invalid checkInID %q: %w", p.CheckInID, err) // permanent
	}

	// Look up accountability style (default to "balanced" on error).
	accountabilityStyle := "balanced"
	if style, err := h.repo.GetAccountabilityStyle(ctx, userID); err == nil && style != "" {
		accountabilityStyle = style
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
		verdict, err := h.classifier.Classify(ctx, p.HabitName)
		if err != nil {
			logx.WithContext(ctx).Errorf("safety classification error: %v", err)
			// Treat safety-classifier errors as transient so we retry rather than
			// skip a potentially-valid event.
			return fmt.Errorf("safety classification: %w", err)
		}
		if verdict.Category != safety.CategorySafe {
			logx.WithContext(ctx).Infof("safety block: category=%s confidence=%.2f reason=%s", verdict.Category, verdict.Confidence, verdict.Reason)
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
		logx.WithContext(ctx).Errorf("AI feedback generation failed: %v", err)
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
			return fmt.Errorf("insert ai_feedback: %w", err)
		}
		if err := h.repo.MarkProcessed(ctx, eventID); err != nil {
			return fmt.Errorf("mark processed: %w", err)
		}
	}

	// Publish feedback generated event.
	if h.pub != nil {
		feedbackEnv, err := events.NewEnvelope(events.TypeCheckInFeedbackGenerated, events.CheckInFeedbackGenerated{
			UserID:    p.UserID,
			CheckInID: checkInID.String(),
			HabitID:   p.HabitID,
			Content:   content,
		})
		if err != nil {
			logx.WithContext(ctx).Errorf("build feedback envelope: %v", err)
		} else if err := h.pub.Publish(ctx, feedbackEnv); err != nil {
			logx.WithContext(ctx).Errorf("publish feedback event: %v", err)
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
	return fmt.Sprintf("completed %d of last %d check-ins", completed, habitChecks)
}

func (h *EventsHandler) sendToDLQ(ctx context.Context, env events.Envelope, raw, reason string, permanent bool) {
	if h.dlqPub == nil {
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
		logx.WithContext(ctx).Errorf("failed to publish to DLQ: %v", err)
	}
}
