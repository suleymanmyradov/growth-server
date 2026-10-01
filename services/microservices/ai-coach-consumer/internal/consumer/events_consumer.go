package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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

// DLQPusher pushes messages to the dead-letter topic.
type DLQPusher interface {
	Publish(ctx context.Context, msg events.DLQMessage) error
}

// TxRunner executes work inside a PostgreSQL transaction.
type TxRunner interface {
	Run(ctx context.Context, userID string, fn func(pgx.Tx) error) error
}

// EventsHandler consumes domain events from the growth.events topic and
// generates AI coaching feedback for check-in events. Outbound events are
// written to ai_coach_event_outbox inside the digest transaction — the svc
// layer runs an outbox relay that drains them to the broker.
type EventsHandler struct {
	repo        *repository.Repository
	txRunner    TxRunner
	ai          AIClient
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
func NewEventsHandler(repo *repository.Repository, aiClient AIClient, classifier SafetyClassifier, opts *EventsHandlerOptions) *EventsHandler {
	h := &EventsHandler{
		repo:        repo,
		ai:          aiClient,
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
	defer func() { timer.ObserveDuration() }()

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

	if events.EventType(env.EventType) == events.TypeUserDeleted {
		logx.WithContext(ctx).Infof("processing user_deleted event: eventID=%s", env.EventID)
		eventsConsumedTotal.WithLabelValues(env.EventType, "processing").Inc()
		err = h.onUserDeleted(ctx, env)
		if err != nil {
			logx.WithContext(ctx).Errorf("error processing user_deleted event: eventID=%s err=%v", env.EventID, err)
			eventsConsumedTotal.WithLabelValues(env.EventType, "retry").Inc()
			return err
		}
		eventsConsumedTotal.WithLabelValues(env.EventType, "success").Inc()
		if err := h.repo.MarkProcessed(ctx, eventID); err != nil {
			logx.WithContext(ctx).Errorf("mark processed: %v", err)
		}
		return nil
	}

	// CheckInCreated events feed the ai_coach_check_ins read model so the
	// digest prompt never reads the client-owned check_ins table (P2). The
	// daily coach digest (triggered by CoachDigestRequested) builds on those
	// mirrored rows; no AI call is made here.
	if events.EventType(env.EventType) == events.TypeCheckInCreated {
		logx.WithContext(ctx).Infof("processing check_in_created event (read-model sync): eventID=%s", env.EventID)
		eventsConsumedTotal.WithLabelValues(env.EventType, "processing").Inc()
		if err := h.onCheckInCreated(ctx, env, eventID); err != nil {
			logx.WithContext(ctx).Errorf("error syncing check-in read model: eventID=%s err=%v", env.EventID, err)
			eventsConsumedTotal.WithLabelValues(env.EventType, "retry").Inc()
			return err
		}
		eventsConsumedTotal.WithLabelValues(env.EventType, "success").Inc()
		return nil
	}

	// CoachingProfileChanged events feed the ai_coach_profiles read model
	// (accountability style) used by the digest prompt.
	if events.EventType(env.EventType) == events.TypeCoachingProfileChanged {
		logx.WithContext(ctx).Infof("processing coaching_profile_changed event: eventID=%s", env.EventID)
		eventsConsumedTotal.WithLabelValues(env.EventType, "processing").Inc()
		if err := h.onCoachingProfileChanged(ctx, env, eventID); err != nil {
			logx.WithContext(ctx).Errorf("error syncing coaching profile read model: eventID=%s err=%v", env.EventID, err)
			eventsConsumedTotal.WithLabelValues(env.EventType, "retry").Inc()
			return err
		}
		eventsConsumedTotal.WithLabelValues(env.EventType, "success").Inc()
		return nil
	}

	if events.EventType(env.EventType) == events.TypeCoachDigestRequested {
		logx.WithContext(ctx).Infof("processing coach_digest_requested event: eventID=%s", env.EventID)
		eventsConsumedTotal.WithLabelValues(env.EventType, "processing").Inc()
		err = h.onCoachDigestRequested(ctx, env, eventID)
		if err != nil {
			if IsTransientError(err) {
				logx.WithContext(ctx).Errorf("transient error processing coach_digest, will retry: eventID=%s err=%v", env.EventID, err)
				eventsConsumedTotal.WithLabelValues(env.EventType, "retry").Inc()
				return err
			}
			logx.WithContext(ctx).Errorf("permanent error processing coach_digest, sending to DLQ: eventID=%s err=%v", env.EventID, err)
			h.sendToDLQ(ctx, env, raw, err.Error(), true)
			eventsConsumedTotal.WithLabelValues(env.EventType, "dlq").Inc()
			return nil
		}
		logx.WithContext(ctx).Infof("coach_digest processed successfully: eventID=%s", env.EventID)
		eventsConsumedTotal.WithLabelValues(env.EventType, "success").Inc()
		return nil
	}

	logx.WithContext(ctx).Infof("ignoring unhandled event: type=%s eventID=%s", env.EventType, env.EventID)
	eventsConsumedTotal.WithLabelValues(env.EventType, "ignored").Inc()
	return nil
}

// onCoachDigestRequested handles a CoachDigestRequested event: fetches all of
// the user's check-ins for the specified date, generates one combined AI
// feedback message covering the entire day, persists it as a single
// ai_feedback row (with NULL check_in_id/habit_id since it covers multiple),
// and publishes a CheckInFeedbackGenerated event so the notifications service
// creates one notification — replacing the old per-check-in feedback that
// spammed users with N notifications for N habits.
func (h *EventsHandler) onCoachDigestRequested(ctx context.Context, env events.Envelope, eventID uuid.UUID) error {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "EventsHandler.onCoachDigestRequested")
	defer span.End()

	if err := ctx.Err(); err != nil {
		return err
	}

	var p events.CoachDigestRequested
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.WithContext(ctx).Errorf("failed to unmarshal CoachDigestRequested payload: eventID=%s err=%v", env.EventID, err)
		return fmt.Errorf("unmarshal CoachDigestRequested: %w", err) // permanent
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		logx.WithContext(ctx).Errorf("invalid userID %q: eventID=%s", p.UserID, env.EventID)
		return fmt.Errorf("invalid userID %q: %w", p.UserID, err) // permanent
	}

	if p.Date == "" {
		logx.WithContext(ctx).Errorf("empty date in CoachDigestRequested: eventID=%s user=%s", env.EventID, p.UserID)
		return fmt.Errorf("empty date") // permanent
	}

	logx.WithContext(ctx).Infof("coach digest requested: user=%s date=%s", p.UserID, p.Date)

	// Fetch all check-ins for the specified date (with habit names).
	checkIns, err := h.repo.GetCheckInsForDate(ctx, userID, p.Date)
	if err != nil {
		logx.WithContext(ctx).Errorf("failed to get check-ins for date: user=%s date=%s err=%v", p.UserID, p.Date, err)
		return fmt.Errorf("get check-ins for date: %w", err) // transient
	}

	if len(checkIns) == 0 {
		// No check-ins today — skip the digest. The missed_check_in reminder
		// handles the "you didn't check in" case. Mark processed so we don't
		// retry.
		logx.WithContext(ctx).Infof("no check-ins for digest: user=%s date=%s", p.UserID, p.Date)
		_ = h.repo.MarkProcessed(ctx, eventID)
		return nil
	}

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

	// Build the digest check-in list for the prompt.
	digestCheckIns := make([]prompts.DigestCheckIn, 0, len(checkIns))
	for _, c := range checkIns {
		dci := prompts.DigestCheckIn{
			HabitName: c.HabitName,
			Status:    c.Status,
		}
		if c.Mood != nil {
			dci.Mood = *c.Mood
		}
		if c.Energy != nil {
			dci.Energy = *c.Energy
		}
		if c.Blocker != nil {
			dci.Blocker = *c.Blocker
		}
		if c.Note != nil {
			dci.Note = *c.Note
		}
		digestCheckIns = append(digestCheckIns, dci)
	}

	// Compute recent 7-day pattern across all habits.
	recentPattern := h.buildDigestRecentPattern(ctx, userID)

	// Safety check on all user-authored free text before it reaches the
	// model. Habit names, blockers, notes, and mood/energy strings are all
	// user-controlled; a single joined classify covers every field in one
	// call. Previously only habit names were screened — crisis, self-harm,
	// and medical content in blocker/note fields reached the prompt verbatim.
	if h.classifier != nil {
		if freeText := digestFreeText(digestCheckIns); freeText != "" {
			verdict, err := safety.ClassifyWithRetry(ctx, h.classifier, freeText)
			if err != nil {
				logx.WithContext(ctx).Errorf("safety classification error, will retry: user=%s err=%v", p.UserID, err)
				return fmt.Errorf("safety classification: %w", err)
			}
			if _, blocked := safety.BlockedResponse(verdict, safety.BlockConfidenceThreshold); blocked {
				// Reason deliberately not logged — may quote sensitive content.
				logx.WithContext(ctx).Infof("safety block on digest input: user=%s category=%s confidence=%.2f", p.UserID, verdict.Category, verdict.Confidence)
				aiSafetyBlockedTotal.Inc()
				_ = h.repo.MarkProcessed(ctx, eventID)
				return nil
			}
			if verdict.Category != safety.CategorySafe {
				logx.WithContext(ctx).Infof("safety flag below threshold on digest input, proceeding: user=%s category=%s confidence=%.2f", p.UserID, verdict.Category, verdict.Confidence)
			}
		}
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	system := prompts.BuildDigestSystemPrompt(accountabilityStyle)
	user := prompts.BuildDigestUserPrompt(prompts.DailyDigestInput{
		CheckIns:            digestCheckIns,
		AccountabilityStyle: accountabilityStyle,
		RecentPattern:       recentPattern,
	})

	// Call AI with a bounded timeout.
	aiCtx, cancel := context.WithTimeout(ctx, h.aiTimeout)
	defer cancel()

	logx.WithContext(ctx).Infof("calling AI for daily digest: user=%s date=%s habits=%d timeout=%s",
		p.UserID, p.Date, len(checkIns), h.aiTimeout)

	aiStart := time.Now()
	resp, err := h.ai.Generate(aiCtx, ai.GenerateRequest{
		ModelProfile: ai.ModelCheap,
		System:       system,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: user},
		},
		Metadata: ai.Metadata{
			UserID:  p.UserID,
			Feature: "daily-digest",
		},
	})
	aiDuration := time.Since(aiStart).Seconds()
	if err != nil {
		aiGenerationDuration.WithLabelValues("error").Observe(aiDuration)
		logx.WithContext(ctx).Errorf("AI digest generation failed: user=%s date=%s duration=%.2fs err=%v", p.UserID, p.Date, aiDuration, err)
		if aiCtx.Err() != nil && ctx.Err() == nil {
			return fmt.Errorf("ai generation timeout: %w", err)
		}
		return fmt.Errorf("ai generation failed: %w", err)
	}
	aiGenerationDuration.WithLabelValues("ok").Observe(aiDuration)
	logx.WithContext(ctx).Infof("AI digest generated: user=%s date=%s model=%s duration=%.2fs tokens=%d",
		p.UserID, p.Date, resp.ModelID, aiDuration, resp.Usage.TotalTokens)

	if err := ctx.Err(); err != nil {
		return err
	}

	content := resp.Message.Content
	feedbackID := uuid.New()

	// The feedback event the notifications service turns into the digest
	// notification. It goes into ai_coach_event_outbox — inside the same
	// transaction as the ai_feedback row + processed marker — so it can't be
	// lost between commit and publish (P1). The relay drains it to the broker.
	feedbackEnv, err := events.NewEnvelope(events.TypeCheckInFeedbackGenerated, events.CheckInFeedbackGenerated{
		UserID:    p.UserID,
		CheckInID: "", // digest covers multiple check-ins
		HabitID:   "",
		Content:   content,
	})
	if err != nil {
		return fmt.Errorf("build feedback envelope: %w", err)
	}
	feedbackEventID, fErr := uuid.Parse(feedbackEnv.EventID)
	if fErr != nil {
		return fmt.Errorf("feedback event ID: %w", fErr)
	}

	// Persist digest feedback (check_in_id and habit_id are NULL for digests),
	// the pending event publication, and the processed marker atomically.
	if h.txRunner != nil {
		err = h.txRunner.Run(ctx, p.UserID, func(tx pgx.Tx) error {
			txRepo := h.repo.WithTx(tx)
			if err := txRepo.InsertAIFeedback(ctx, db.InsertAIFeedbackParams{
				ID:        feedbackID,
				UserID:    userID,
				CheckInID: nil, // digest covers multiple check-ins
				HabitID:   nil,
				Content:   content,
				Model:     resp.ModelID,
			}); err != nil {
				return fmt.Errorf("insert ai_feedback: %w", err)
			}
			if err := txRepo.EnqueueEvent(ctx, feedbackEventID, feedbackEnv.EventType, feedbackEnv.Payload, feedbackEnv.OccurredAt); err != nil {
				return fmt.Errorf("enqueue feedback event: %w", err)
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
		if err := h.repo.InsertAIFeedback(ctx, db.InsertAIFeedbackParams{
			ID:        feedbackID,
			UserID:    userID,
			CheckInID: nil,
			HabitID:   nil,
			Content:   content,
			Model:     resp.ModelID,
		}); err != nil {
			logx.WithContext(ctx).Errorf("insert ai_feedback failed: user=%s eventID=%s err=%v", p.UserID, env.EventID, err)
			return fmt.Errorf("insert ai_feedback: %w", err)
		}
		if err := h.repo.EnqueueEvent(ctx, feedbackEventID, feedbackEnv.EventType, feedbackEnv.Payload, feedbackEnv.OccurredAt); err != nil {
			logx.WithContext(ctx).Errorf("enqueue feedback event failed: user=%s eventID=%s err=%v", p.UserID, env.EventID, err)
			return fmt.Errorf("enqueue feedback event: %w", err)
		}
		if err := h.repo.MarkProcessed(ctx, eventID); err != nil {
			logx.WithContext(ctx).Errorf("mark processed failed: user=%s eventID=%s err=%v", p.UserID, env.EventID, err)
			return fmt.Errorf("mark processed: %w", err)
		}
	}

	logx.WithContext(ctx).Infof("digest persisted: user=%s date=%s feedbackID=%s", p.UserID, p.Date, feedbackID)

	logx.WithContext(ctx).Infof("generated daily digest for user %s date %s", p.UserID, p.Date)
	return nil
}

// onCheckInCreated mirrors a check_in_created event into the consumer-owned
// ai_coach_check_ins read model (P2). Upserts are keyed on check_in_id so
// re-check-ins (missed → completed) keep the row truthful. The row +
// processed_events marker commit atomically when a txRunner is available.
func (h *EventsHandler) onCheckInCreated(ctx context.Context, env events.Envelope, eventID uuid.UUID) error {
	var p events.CheckInCreated
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		logx.WithContext(ctx).Errorf("unmarshal CheckInCreated: eventID=%s err=%v", env.EventID, err)
		// Permanent — can't parse; mark processed so it doesn't retry forever.
		_ = h.repo.MarkProcessed(ctx, eventID)
		return nil
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		_ = h.repo.MarkProcessed(ctx, eventID)
		return nil
	}
	checkInID, err := uuid.Parse(p.CheckInID)
	if err != nil {
		_ = h.repo.MarkProcessed(ctx, eventID)
		return nil
	}
	habitID, err := uuid.Parse(p.HabitID)
	if err != nil {
		_ = h.repo.MarkProcessed(ctx, eventID)
		return nil
	}

	localDate := p.LocalDate
	if localDate == "" {
		// Older publishers may omit local_date; fall back to the event date
		// (UTC). Newer publishers always send it.
		localDate = env.OccurredAt.Format("2006-01-02")
	}

	arg := db.UpsertCoachCheckInParams{
		CheckInID:  checkInID,
		UserID:     userID,
		HabitID:    habitID,
		HabitName:  p.HabitName,
		Status:     p.Status,
		Mood:       strPtrOrNil(p.Mood),
		Energy:     strPtrOrNil(p.Energy),
		Blocker:    strPtrOrNil(p.Blocker),
		Note:       strPtrOrNil(p.Note),
		LocalDate:  localDate,
		OccurredAt: env.OccurredAt,
	}

	if h.txRunner != nil {
		err = h.txRunner.Run(ctx, p.UserID, func(tx pgx.Tx) error {
			txRepo := h.repo.WithTx(tx)
			if err := txRepo.UpsertCoachCheckIn(ctx, arg); err != nil {
				return err
			}
			return txRepo.MarkProcessed(ctx, eventID)
		})
		return err
	}
	if err := h.repo.UpsertCoachCheckIn(ctx, arg); err != nil {
		return err
	}
	return h.repo.MarkProcessed(ctx, eventID)
}

// onCoachingProfileChanged mirrors the accountability style into
// ai_coach_profiles so the digest prompt can read it without touching the
// client-owned coaching_profiles table.
func (h *EventsHandler) onCoachingProfileChanged(ctx context.Context, env events.Envelope, eventID uuid.UUID) error {
	var p events.CoachingProfileChanged
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		_ = h.repo.MarkProcessed(ctx, eventID)
		return nil
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil || p.AccountabilityStyle == "" {
		_ = h.repo.MarkProcessed(ctx, eventID)
		return nil
	}
	if h.txRunner != nil {
		return h.txRunner.Run(ctx, p.UserID, func(tx pgx.Tx) error {
			txRepo := h.repo.WithTx(tx)
			if err := txRepo.UpsertCoachProfile(ctx, userID, p.AccountabilityStyle); err != nil {
				return err
			}
			return txRepo.MarkProcessed(ctx, eventID)
		})
	}
	if err := h.repo.UpsertCoachProfile(ctx, userID, p.AccountabilityStyle); err != nil {
		return err
	}
	return h.repo.MarkProcessed(ctx, eventID)
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// buildDigestRecentPattern computes a 7-day pattern across ALL habits (not
// per-habit like the old buildRecentPattern). Returns a summary string like
// "completed 12 of last 20 check-ins".
func (h *EventsHandler) buildDigestRecentPattern(ctx context.Context, userID uuid.UUID) string {
	if err := ctx.Err(); err != nil {
		return ""
	}
	now := time.Now().UTC()
	start := now.AddDate(0, 0, -7)
	checkIns, err := h.repo.GetCheckInsForWeek(ctx, userID, start, now)
	if err != nil {
		logx.WithContext(ctx).Infof("failed to get check-ins for digest pattern, returning empty: user=%s err=%v", userID, err)
		return ""
	}
	if len(checkIns) == 0 {
		return ""
	}
	completed := 0
	for _, c := range checkIns {
		if c.Status == "completed" {
			completed++
		}
	}
	return fmt.Sprintf("completed %d of last %d check-ins across all habits", completed, len(checkIns))
}

// digestFreeText joins every user-authored free-text field that gets
// interpolated into the digest prompt so they can be screened together in a
// single classifier call.
func digestFreeText(checkIns []prompts.DigestCheckIn) string {
	var parts []string
	for _, c := range checkIns {
		for _, s := range []string{c.HabitName, c.Status, c.Mood, c.Energy, c.Blocker, c.Note} {
			if t := strings.TrimSpace(s); t != "" {
				parts = append(parts, t)
			}
		}
	}
	return strings.Join(parts, "\n")
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

// onUserDeleted cleans up this service's own tables for a deleted user:
// ai_feedback plus the ai_coach_* read models. Conversation tables are owned
// by the ai-coach service, which runs its own user_deleted cleanup (P2).
func (h *EventsHandler) onUserDeleted(ctx context.Context, env events.Envelope) error {
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

	logx.WithContext(ctx).Infof("cleaning up ai_feedback and read models for user %s", userID)
	if err := h.repo.DeleteAIFeedbackByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete ai_feedback: %w", err)
	}
	if err := h.repo.DeleteCoachCheckInsByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete ai_coach_check_ins: %w", err)
	}
	if err := h.repo.DeleteCoachProfileByUser(ctx, userID); err != nil {
		return fmt.Errorf("delete ai_coach_profiles: %w", err)
	}
	return nil
}
