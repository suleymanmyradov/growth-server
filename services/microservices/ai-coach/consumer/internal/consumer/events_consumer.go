package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
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

// EventsHandler consumes domain events from the growth.events topic and
// generates AI coaching feedback for check-in events.
type EventsHandler struct {
	repo      *repository.Repository
	ai        AIClient
	pub       Publisher
	classifier SafetyClassifier
}

// NewEventsHandler creates a handler with the given dependencies.
func NewEventsHandler(repo *repository.Repository, aiClient AIClient, pub Publisher, classifier SafetyClassifier) *EventsHandler {
	return &EventsHandler{repo: repo, ai: aiClient, pub: pub, classifier: classifier}
}

// Consume is the kq.ConsumeHandler callback.
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

	// Idempotency check.
	processed, err := h.repo.IsProcessed(ctx, eventID)
	if err != nil {
		return fmt.Errorf("idempotency check: %w", err)
	}
	if processed {
		logx.WithContext(ctx).Infof("duplicate event %s, skipping", env.EventID)
		return nil
	}

	if events.EventType(env.EventType) != events.TypeCheckInCreated {
		return nil
	}

	return h.onCheckInCreated(ctx, env, eventID)
}

func (h *EventsHandler) onCheckInCreated(ctx context.Context, env events.Envelope, eventID uuid.UUID) error {
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

	checkInID, err := uuid.Parse(p.CheckInID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid checkInID %q: %v", p.CheckInID, err)
		return nil
	}

	// Look up accountability style (default to "balanced" on error).
	accountabilityStyle := "balanced"
	if style, err := h.repo.GetAccountabilityStyle(ctx, userID); err == nil && style != "" {
		accountabilityStyle = style
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
		} else if verdict.Category != safety.CategorySafe {
			logx.WithContext(ctx).Infof("safety block: category=%s confidence=%.2f reason=%s", verdict.Category, verdict.Confidence, verdict.Reason)
			_ = h.repo.MarkProcessed(ctx, eventID)
			return nil
		}
	}

	system := prompts.BuildSystemPrompt(accountabilityStyle)
	user := prompts.BuildUserPrompt(promptInput)

	// Call AI.
	resp, err := h.ai.Generate(ctx, ai.GenerateRequest{
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
	if err != nil {
		// AI errors are usually permanent for that prompt; don't retry forever.
		logx.WithContext(ctx).Errorf("AI feedback generation failed: %v", err)
		// Still mark as processed so we don't retry.
		_ = h.repo.MarkProcessed(ctx, eventID)
		return nil
	}

	content := resp.Message.Content

	// Persist feedback.
	feedbackID := uuid.New()
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

	// Mark event as processed.
	if err := h.repo.MarkProcessed(ctx, eventID); err != nil {
		logx.WithContext(ctx).Errorf("mark processed: %v", err)
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
