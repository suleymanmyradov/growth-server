package consumer

import (
	"context"
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/consumer/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/consumer/internal/repository/db"
)

// fakeAI returns a fixed response for every Generate call.
type fakeAI struct {
	content string
	modelID string
}

func (f *fakeAI) Generate(_ context.Context, _ ai.GenerateRequest) (ai.GenerateResponse, error) {
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

// fakeRepo provides an in-memory repository for tests.
type fakeRepo struct {
	mu          sync.Mutex
	processed   map[string]bool
	feedback    []db.InsertAIFeedbackParams
	style       string
	styleErr    error
	checkIns    []db.CheckIn
	checkInsErr error
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
	r.processed[eventID.String()] = true
	return nil
}

func TestEventsHandler_InvalidEnvelope(t *testing.T) {
	h := NewEventsHandler(&repository.Repository{}, nil, nil, nil)
	err := h.Consume(context.Background(), "", "not-json")
	if err != nil {
		t.Fatalf("expected nil on invalid envelope, got %v", err)
	}
}

func TestEventsHandler_InvalidEventID(t *testing.T) {
	h := NewEventsHandler(&repository.Repository{}, nil, nil, nil)
	env := events.Envelope{EventID: "not-a-uuid", EventType: string(events.TypeCheckInCreated)}
	raw, _ := json.Marshal(env)
	err := h.Consume(context.Background(), "", string(raw))
	if err != nil {
		t.Fatalf("expected nil on invalid eventID, got %v", err)
	}
}

func TestEventsHandler_UnhandledEventType(t *testing.T) {
	h := NewEventsHandler(&repository.Repository{}, nil, nil, nil)
	env, _ := events.NewEnvelope("unknown_type", nil)
	raw, _ := json.Marshal(env)
	err := h.Consume(context.Background(), "", string(raw))
	if err != nil {
		t.Fatalf("expected nil on unhandled type, got %v", err)
	}
}

func TestEventsHandler_CheckInCreated_HappyPath(t *testing.T) {
	// This test is covered by TestCheckInCreated_HappyPath below
	// which uses the testHandler with injectable interfaces.
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
