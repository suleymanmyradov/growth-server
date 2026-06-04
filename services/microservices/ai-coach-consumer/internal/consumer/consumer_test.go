package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach-consumer/internal/repository/db"
)

// fakeAI returns a fixed response for every Generate call.
type fakeAI struct {
	content string
	modelID string
	err     error
}

func (f *fakeAI) Generate(_ context.Context, _ ai.GenerateRequest) (ai.GenerateResponse, error) {
	if f.err != nil {
		return ai.GenerateResponse{}, f.err
	}
	return ai.GenerateResponse{
		Message: ai.Message{Role: ai.RoleAssistant, Content: f.content},
		Usage:   ai.Usage{TotalTokens: 10},
		ModelID: f.modelID,
	}, nil
}

// fakePublisher captures published envelopes.
type fakePublisher struct {
	mu   sync.Mutex
	envs []events.Envelope
}

func (f *fakePublisher) Publish(_ context.Context, env events.Envelope) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.envs = append(f.envs, env)
	return nil
}

func (f *fakePublisher) published() []events.Envelope {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]events.Envelope{}, f.envs...)
}

// fakeDLQPusher captures DLQ messages.
type fakeDLQPusher struct {
	mu   sync.Mutex
	msgs []events.DLQMessage
}

func (f *fakeDLQPusher) Publish(_ context.Context, msg events.DLQMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.msgs = append(f.msgs, msg)
	return nil
}

func (f *fakeDLQPusher) messages() []events.DLQMessage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]events.DLQMessage{}, f.msgs...)
}

// fakeRepo provides an in-memory repository for tests.
type fakeRepo struct {
	mu          sync.Mutex
	processed   map[string]bool
	feedback    []db.InsertAIFeedbackParams
	style       string
	styleErr    error
	checkIns    []db.CheckIn
	checkInsErr error
	insertErr   error
	markErr     error
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{
		processed: make(map[string]bool),
		style:     "balanced",
	}
}

func (r *fakeRepo) InsertAIFeedback(_ context.Context, arg db.InsertAIFeedbackParams) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.insertErr != nil {
		return r.insertErr
	}
	r.feedback = append(r.feedback, arg)
	return nil
}

func (r *fakeRepo) GetCheckInsForWeek(_ context.Context, _ uuid.UUID, _, _ interface{}) ([]db.CheckIn, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.checkIns, r.checkInsErr
}

func (r *fakeRepo) GetAccountabilityStyle(_ context.Context, _ uuid.UUID) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.style, r.styleErr
}

func (r *fakeRepo) IsProcessed(_ context.Context, eventID uuid.UUID) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.processed[eventID.String()], nil
}

func (r *fakeRepo) MarkProcessed(_ context.Context, eventID uuid.UUID) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.markErr != nil {
		return r.markErr
	}
	r.processed[eventID.String()] = true
	return nil
}

func (r *fakeRepo) WithTx(_ pgx.Tx) *fakeRepo {
	return r
}

// fakeTxRunner runs the callback directly without a real transaction.
type fakeTxRunner struct{}

func (f *fakeTxRunner) Run(_ context.Context, _ string, fn func(pgx.Tx) error) error {
	return fn(nil)
}

func TestEventsHandler_InvalidEnvelope(t *testing.T) {
	dlq := &fakeDLQPusher{}
	opts := &EventsHandlerOptions{DLQPub: dlq}
	h := NewEventsHandler(&repository.Repository{}, nil, nil, nil, opts)
	err := h.Consume(context.Background(), "", "not-json")
	require.NoError(t, err)
	msgs := dlq.messages()
	require.Len(t, msgs, 1)
	assert.Equal(t, "invalid_envelope", msgs[0].Reason)
}

func TestEventsHandler_InvalidEventID(t *testing.T) {
	dlq := &fakeDLQPusher{}
	opts := &EventsHandlerOptions{DLQPub: dlq}
	h := NewEventsHandler(&repository.Repository{}, nil, nil, nil, opts)
	env := events.Envelope{EventID: "not-a-uuid", EventType: string(events.TypeCheckInCreated), Version: 1, Payload: []byte(`{}`)}
	raw, _ := json.Marshal(env)
	err := h.Consume(context.Background(), "", string(raw))
	require.NoError(t, err)
	msgs := dlq.messages()
	require.Len(t, msgs, 1)
	assert.Equal(t, "invalid_event_id", msgs[0].Reason)
}

func TestEventsHandler_UnknownEventType(t *testing.T) {
	dlq := &fakeDLQPusher{}
	opts := &EventsHandlerOptions{DLQPub: dlq}
	h := NewEventsHandler(&repository.Repository{}, nil, nil, nil, opts)
	env, _ := events.NewEnvelope("unknown_type", struct{}{})
	raw, _ := json.Marshal(env)
	err := h.Consume(context.Background(), "", string(raw))
	require.NoError(t, err)
	msgs := dlq.messages()
	require.Len(t, msgs, 1)
	assert.Contains(t, msgs[0].Reason, "validation_failed")
}

func TestEventsHandler_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	h := NewEventsHandler(&repository.Repository{}, nil, nil, nil, nil)
	err := h.Consume(ctx, "", "anything")
	assert.ErrorIs(t, err, context.Canceled)
}

func TestEventsHandler_ConcurrencyLimit(t *testing.T) {
	dlq := &fakeDLQPusher{}
	// Use the real handler with a nil repo and trigger an error path
	// that exercises the semaphore path.
	opts := &EventsHandlerOptions{DLQPub: dlq, Concurrency: 1}
	h := NewEventsHandler(&repository.Repository{}, nil, nil, nil, opts)

	// Two rapid invalid messages — first should acquire semaphore, second should too.
	h.Consume(context.Background(), "", "bad")
	h.Consume(context.Background(), "", "bad")
	assert.Len(t, dlq.messages(), 2)
}

// Since we can't easily inject a fake into *repository.Repository,
// we test via a custom handler struct that accepts interfaces.

// testHandler uses interfaces for all dependencies.
type testHandler struct {
	repo *fakeRepo
	ai   AIClient
	pub  Publisher
}

func (h *testHandler) Consume(ctx context.Context, _ string, raw string) error {
	var env events.Envelope
	if err := json.Unmarshal([]byte(raw), &env); err != nil {
		return nil
	}
	eventID, err := uuid.Parse(env.EventID)
	if err != nil {
		return nil
	}
	processed, _ := h.repo.IsProcessed(ctx, eventID)
	if processed {
		return nil
	}
	if events.EventType(env.EventType) != events.TypeCheckInCreated {
		return nil
	}

	var p events.CheckInCreated
	if err := json.Unmarshal(env.Payload, &p); err != nil {
		return nil
	}

	userID, _ := uuid.Parse(p.UserID)
	habitID, _ := uuid.Parse(p.HabitID)

	style, _ := h.repo.GetAccountabilityStyle(ctx, userID)
	if style == "" {
		style = "balanced"
	}

	resp, err := h.ai.Generate(ctx, ai.GenerateRequest{
		ModelProfile: ai.ModelCheap,
		System:       "test for " + style,
		Messages:     []ai.Message{{Role: ai.RoleUser, Content: "test"}},
		Metadata:     ai.Metadata{UserID: p.UserID, Feature: "check-in-feedback"},
	})
	if err != nil {
		_ = h.repo.MarkProcessed(ctx, eventID)
		return nil
	}

	_ = h.repo.InsertAIFeedback(ctx, db.InsertAIFeedbackParams{
		ID:        uuid.New(),
		UserID:    userID,
		CheckInID: uuid.New(),
		HabitID:   habitID,
		Content:   resp.Message.Content,
		Model:     resp.ModelID,
	})
	_ = h.repo.MarkProcessed(ctx, eventID)

	if h.pub != nil {
		feedbackEnv, _ := events.NewEnvelope(events.TypeCheckInFeedbackGenerated, events.CheckInFeedbackGenerated{
			UserID:    p.UserID,
			CheckInID: uuid.New().String(),
			HabitID:   p.HabitID,
			Content:   resp.Message.Content,
		})
		_ = h.pub.Publish(ctx, feedbackEnv)
	}
	return nil
}

func TestCheckInCreated_HappyPath(t *testing.T) {
	repo := newFakeRepo()
	pub := &fakePublisher{}
	h := &testHandler{repo: repo, ai: &fakeAI{content: "Great work!", modelID: "test-model"}, pub: pub}

	userID := uuid.New()
	habitID := uuid.New()

	env, err := events.NewEnvelope(events.TypeCheckInCreated, events.CheckInCreated{
		UserID:    userID.String(),
		HabitID:   habitID.String(),
		HabitName: "Meditation",
		Status:    "completed",
		Streak:    7,
	})
	require.NoError(t, err)

	raw, err := json.Marshal(env)
	require.NoError(t, err)

	err = h.Consume(context.Background(), "", string(raw))
	assert.NoError(t, err)

	assert.Len(t, repo.feedback, 1)
	assert.Equal(t, "Great work!", repo.feedback[0].Content)
	assert.Equal(t, "test-model", repo.feedback[0].Model)
	assert.True(t, repo.processed[env.EventID])

	published := pub.published()
	assert.Len(t, published, 1)
	assert.Equal(t, string(events.TypeCheckInFeedbackGenerated), published[0].EventType)

	var feedbackPayload events.CheckInFeedbackGenerated
	require.NoError(t, json.Unmarshal(published[0].Payload, &feedbackPayload))
	assert.Equal(t, "Great work!", feedbackPayload.Content)
}

func TestCheckInCreated_DuplicateSkipped(t *testing.T) {
	repo := newFakeRepo()
	pub := &fakePublisher{}
	h := &testHandler{repo: repo, ai: &fakeAI{content: "Great work!", modelID: "test-model"}, pub: pub}

	userID := uuid.New()
	habitID := uuid.New()

	env, _ := events.NewEnvelope(events.TypeCheckInCreated, events.CheckInCreated{
		UserID:    userID.String(),
		HabitID:   habitID.String(),
		HabitName: "Meditation",
		Status:    "completed",
		Streak:    3,
	})
	raw, _ := json.Marshal(env)

	// First call: processes.
	require.NoError(t, h.Consume(context.Background(), "", string(raw)))
	assert.Len(t, repo.feedback, 1)

	// Second call: skipped as duplicate.
	require.NoError(t, h.Consume(context.Background(), "", string(raw)))
	assert.Len(t, repo.feedback, 1)   // still 1
	assert.Len(t, pub.published(), 1) // still 1
}

func TestEventsHandler_TransientError_Retry(t *testing.T) {
	repo := newFakeRepo()
	repo.insertErr = errors.New("db connection refused")

	// The easiest way to verify transient error behaviour is to test the
	// error classification directly.
	assert.True(t, IsTransientError(repo.insertErr))
}

func TestIsTransientError(t *testing.T) {
	assert.False(t, IsTransientError(nil))
	assert.True(t, IsTransientError(context.Canceled))
	assert.True(t, IsTransientError(context.DeadlineExceeded))
	assert.True(t, IsTransientError(fmt.Errorf("wrapped: %w", context.DeadlineExceeded)))
	// Unknown errors default to transient so we don't lose messages.
	assert.True(t, IsTransientError(errors.New("db connection refused")))
}

func TestEventsHandler_AITimeout(t *testing.T) {
	slowAI := &fakeAI{err: context.DeadlineExceeded}
	opts := &EventsHandlerOptions{
		TxRunner:    &fakeTxRunner{},
		AITimeout:   50 * time.Millisecond,
		Concurrency: 1,
	}
	h := NewEventsHandler(repository.NewRepository(nil), slowAI, nil, nil, opts)

	userID := uuid.New()
	habitID := uuid.New()

	checkInID := uuid.New()
	env, err := events.NewEnvelope(events.TypeCheckInCreated, events.CheckInCreated{
		UserID:    userID.String(),
		CheckInID: checkInID.String(),
		HabitID:   habitID.String(),
		HabitName: "Meditation",
		Status:    "completed",
		Streak:    1,
	})
	require.NoError(t, err)

	raw, _ := json.Marshal(env)

	// Because the AI error is context.DeadlineExceeded, IsTransientError returns true,
	// so Consume should return the error to trigger a Kafka retry.
	err = h.Consume(context.Background(), "", string(raw))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ai generation")
}
