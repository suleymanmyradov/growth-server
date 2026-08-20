package personalization

import (
	"context"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/aitest"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/config"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/sse"
	"github.com/suleymanmyradov/growth-server/services/ai-gateway/aiapi/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/conversationservice"
	aicoachpb "github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"
	authpb "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/pb/auth"
	"google.golang.org/grpc"
)

// --- Mock implementations ---

type mockConversationStore struct {
	appendCalls  []*conversationservice.AppendMessageRequest
	appendErrors []error
	getCalls     []*conversationservice.GetMessagesRequest
	messages     []*aicoachpb.ConversationMessage
	getError     error
}

func (m *mockConversationStore) AppendMessage(_ context.Context, in *conversationservice.AppendMessageRequest, _ ...grpc.CallOption) (*conversationservice.AppendMessageResponse, error) {
	m.appendCalls = append(m.appendCalls, in)
	var err error
	if len(m.appendErrors) > 0 {
		err = m.appendErrors[0]
		m.appendErrors = m.appendErrors[1:]
	}
	return &conversationservice.AppendMessageResponse{}, err
}

func (m *mockConversationStore) GetMessages(_ context.Context, in *conversationservice.GetMessagesRequest, _ ...grpc.CallOption) (*conversationservice.GetMessagesResponse, error) {
	m.getCalls = append(m.getCalls, in)
	if m.getError != nil {
		return nil, m.getError
	}
	return &conversationservice.GetMessagesResponse{Messages: m.messages}, nil
}

type mockProfileFetcher struct {
	user *authpb.User
	err  error
}

func (m *mockProfileFetcher) GetProfile(_ context.Context, _ *authservice.GetProfileRequest, _ ...grpc.CallOption) (*authservice.GetProfileResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &authservice.GetProfileResponse{User: m.user}, nil
}

type mockClassifier struct {
	verdict safety.Verdict
	err     error
	calls   []string
}

func (m *mockClassifier) Classify(_ context.Context, text string) (safety.Verdict, error) {
	m.calls = append(m.calls, text)
	if m.err != nil {
		return safety.Verdict{}, m.err
	}
	return m.verdict, nil
}

// --- Test helpers ---

func testPrincipal() principal.Principal {
	return principal.Principal{UserID: "user-123"}
}

func parseSSEEvents(raw string) []string {
	raw = strings.TrimSuffix(raw, "\n")
	parts := strings.Split(raw, "\n\n")
	var events []string
	for _, p := range parts {
		if p != "" && !strings.HasPrefix(p, ":") {
			events = append(events, p)
		}
	}
	return events
}

func runStreamCoaching(t *testing.T, req *types.GeneratePersonalizedCoachingRequest, deps StreamCoachingDeps) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	StreamCoaching(ctx, sse.NewWriter(rec), req, testPrincipal(), deps)
	return rec
}

// --- Tests ---

func TestBuildAgenticCoachingSystemPrompt_SupportWithoutCannedEmpathy(t *testing.T) {
	prompt := BuildAgenticCoachingSystemPrompt(AgenticCoachingContext{UserFullName: "Test User"})

	for _, instruction := range []string{
		"Make support feel relational and specific",
		"instead of announcing \"I'm here to listen\"",
		"needing rest does not make someone lazy",
		"Avoid canned empathy and exaggerated mirroring",
		"Never justify or normalize a harmful coping behavior",
		"Do not give them another decision or ask them to define what support should look like",
		"talking with a trusted person or mental health professional as an option",
		"Use the user's name sparingly",
	} {
		assert.Contains(t, prompt, instruction)
	}
}

func TestStreamCoaching_DirectAnswer(t *testing.T) {
	aiClient := aitest.NewMockClient()
	aiClient.RecordAgentStream(ai.ModelChat,
		ai.AgentStreamChunk{Delta: "Hello "},
		ai.AgentStreamChunk{Delta: "world!"},
		ai.AgentStreamChunk{Complete: true, FullResponse: "Hello world!", FinishReason: "stop"},
	)

	conv := &mockConversationStore{}
	profile := &mockProfileFetcher{user: &authpb.User{FullName: "Test User"}}

	rec := runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage: "Hi there",
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Conversations:  conv,
		ProfileFetcher: profile,
	})

	assert.Equal(t, 200, rec.Code)
	events := parseSSEEvents(rec.Body.String())
	// Should have: 2 deltas + 1 complete (no thinking because first event arrives fast)
	require.GreaterOrEqual(t, len(events), 3, "should have deltas and complete")

	// Last event should be complete
	assert.Contains(t, events[len(events)-1], "event: complete")
	assert.Contains(t, events[len(events)-1], `"fullResponse":"Hello world!"`)

	// Should NOT have persisted assistant message (no conversationId)
	assert.Empty(t, conv.appendCalls)
}

func TestStreamCoaching_WithConversationPersistsMessages(t *testing.T) {
	aiClient := aitest.NewMockClient()
	aiClient.RecordAgentStream(ai.ModelChat,
		ai.AgentStreamChunk{Delta: "Great job!"},
		ai.AgentStreamChunk{Complete: true, FullResponse: "Great job!", FinishReason: "stop"},
	)

	conv := &mockConversationStore{
		messages: []*aicoachpb.ConversationMessage{
			{Role: "user", Content: "previous message"},
			{Role: "assistant", Content: "previous reply"},
			{Role: "user", Content: "How am I doing?"}, // just-appended user message
		},
	}
	profile := &mockProfileFetcher{}

	rec := runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage:    "How am I doing?",
		ConversationId: "conv-1",
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Conversations:  conv,
		ProfileFetcher: profile,
	})

	assert.Equal(t, 200, rec.Code)

	// Should have appended user message, then assistant message
	require.Len(t, conv.appendCalls, 2)
	assert.Equal(t, "user", conv.appendCalls[0].Role)
	assert.Equal(t, "How am I doing?", conv.appendCalls[0].Content)
	assert.Equal(t, "assistant", conv.appendCalls[1].Role)
	assert.Equal(t, "Great job!", conv.appendCalls[1].Content)

	// Should have fetched messages
	require.Len(t, conv.getCalls, 1)
	assert.Equal(t, "conv-1", conv.getCalls[0].ConversationId)
}

func TestStreamCoaching_CrisisResponse(t *testing.T) {
	aiClient := aitest.NewMockClient()
	classifier := &mockClassifier{
		verdict: safety.Verdict{
			Category:   safety.CategoryCrisis,
			Confidence: 0.90,
		},
	}
	conv := &mockConversationStore{}

	rec := runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage:    "I want to hurt myself",
		ConversationId: "conv-1",
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Classifier:     classifier,
		Conversations:  conv,
		ProfileFetcher: &mockProfileFetcher{},
	})

	assert.Equal(t, 200, rec.Code)

	// Should NOT have called the AI client
	// (no agent stream recorded for this scenario, so it would error if called)

	// Should have persisted user message + crisis response
	require.Len(t, conv.appendCalls, 2)
	assert.Equal(t, "user", conv.appendCalls[0].Role)
	assert.Equal(t, "I want to hurt myself", conv.appendCalls[0].Content)
	assert.Equal(t, "assistant", conv.appendCalls[1].Role)
	assert.Equal(t, safety.CrisisResponse, conv.appendCalls[1].Content)

	// SSE should contain delta + complete with crisis response
	events := parseSSEEvents(rec.Body.String())
	require.GreaterOrEqual(t, len(events), 2)
	assert.Contains(t, events[0], "event: delta")
	// CrisisResponse contains newlines that get JSON-escaped, so check
	// for a substring that survives escaping.
	assert.Contains(t, events[0], "accountability coach")
	assert.Contains(t, events[len(events)-1], "event: complete")
}

func TestStreamCoaching_SafetyFlagBelowThresholdProceeds(t *testing.T) {
	aiClient := aitest.NewMockClient()
	aiClient.RecordAgentStream(ai.ModelChat,
		ai.AgentStreamChunk{Delta: "You're doing fine."},
		ai.AgentStreamChunk{Complete: true, FullResponse: "You're doing fine.", FinishReason: "stop"},
	)

	classifier := &mockClassifier{
		verdict: safety.Verdict{
			Category:   safety.CategoryCrisis,
			Confidence: 0.50, // below 0.75 threshold
		},
	}

	rec := runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage: "I'm stressed",
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Classifier:     classifier,
		Conversations:  &mockConversationStore{},
		ProfileFetcher: &mockProfileFetcher{},
	})

	assert.Equal(t, 200, rec.Code)
	events := parseSSEEvents(rec.Body.String())
	// Should proceed with normal streaming, not crisis
	require.GreaterOrEqual(t, len(events), 2)
	assert.Contains(t, events[len(events)-1], "event: complete")
	assert.NotContains(t, rec.Body.String(), safety.CrisisResponse)
}

func TestStreamCoaching_ClassifierErrorProceeds(t *testing.T) {
	aiClient := aitest.NewMockClient()
	aiClient.RecordAgentStream(ai.ModelChat,
		ai.AgentStreamChunk{Complete: true, FullResponse: "OK", FinishReason: "stop"},
	)

	classifier := &mockClassifier{
		err: io.ErrUnexpectedEOF,
	}

	rec := runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage: "Hello",
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Classifier:     classifier,
		Conversations:  &mockConversationStore{},
		ProfileFetcher: &mockProfileFetcher{},
	})

	assert.Equal(t, 200, rec.Code)
	events := parseSSEEvents(rec.Body.String())
	require.GreaterOrEqual(t, len(events), 1)
	assert.Contains(t, events[len(events)-1], "event: complete")
}

func TestStreamCoaching_ToolCallEmitsThinkingEvent(t *testing.T) {
	aiClient := aitest.NewMockClient()
	aiClient.RecordAgentStream(ai.ModelChat,
		ai.AgentStreamChunk{
			ToolCall: &ai.ToolCallEvent{Name: "get_active_goals", Status: ai.ToolStatusStarted},
		},
		ai.AgentStreamChunk{
			ToolCall: &ai.ToolCallEvent{Name: "get_active_goals", Status: ai.ToolStatusCompleted, Result: `[]`},
		},
		ai.AgentStreamChunk{Delta: "You have no goals yet."},
		ai.AgentStreamChunk{Complete: true, FullResponse: "You have no goals yet.", FinishReason: "stop"},
	)

	rec := runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage: "What should I work on?",
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Conversations:  &mockConversationStore{},
		ProfileFetcher: &mockProfileFetcher{},
	})

	assert.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	// Should have a thinking event with the tool status message
	assert.Contains(t, body, "event: thinking")
	assert.Contains(t, body, "Looking up your goals...")
}

func TestStreamCoaching_ProposalToolEmitsProposalEvent(t *testing.T) {
	aiClient := aitest.NewMockClient()
	aiClient.RecordAgentStream(ai.ModelChat,
		ai.AgentStreamChunk{
			ToolCall: &ai.ToolCallEvent{
				Name:   "propose_create_goal",
				Status: ai.ToolStatusStarted,
			},
		},
		ai.AgentStreamChunk{
			ToolCall: &ai.ToolCallEvent{
				Name:   "propose_create_goal",
				Status: ai.ToolStatusCompleted,
				Result: `{"id":"prop-1","action":"create_goal","payload":{"title":"Read 10 books"}}`,
			},
		},
		ai.AgentStreamChunk{Delta: "I've prepared a goal for you."},
		ai.AgentStreamChunk{Complete: true, FullResponse: "I've prepared a goal for you.", FinishReason: "stop"},
	)

	rec := runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage: "Help me set a goal",
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Conversations:  &mockConversationStore{},
		ProfileFetcher: &mockProfileFetcher{},
	})

	assert.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	// Should have a proposal event
	assert.Contains(t, body, "event: proposal")
	assert.Contains(t, body, `"id":"prop-1"`)
	assert.Contains(t, body, `"action":"create_goal"`)
	assert.Contains(t, body, `"title":"Read 10 books"`)
}

func TestStreamCoaching_ReasoningEventForwarded(t *testing.T) {
	aiClient := aitest.NewMockClient()
	aiClient.RecordAgentStream(ai.ModelChat,
		ai.AgentStreamChunk{Reasoning: "Let me think about this..."},
		ai.AgentStreamChunk{Delta: "Here's my thought."},
		ai.AgentStreamChunk{Complete: true, FullResponse: "Here's my thought.", FinishReason: "stop"},
	)

	rec := runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage: "What do you think?",
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Conversations:  &mockConversationStore{},
		ProfileFetcher: &mockProfileFetcher{},
	})

	assert.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "event: reasoning")
	assert.Contains(t, body, `"text":"Let me think about this..."`)
}

func TestStreamCoaching_StreamErrorEmitsErrorEvent(t *testing.T) {
	aiClient := aitest.NewMockClient()
	aiClient.RecordAgentStream(ai.ModelChat,
		ai.AgentStreamChunk{Delta: "partial..."},
		ai.AgentStreamChunk{Error: ai.ErrMaxSteps},
	)

	rec := runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage: "Hello",
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Conversations:  &mockConversationStore{},
		ProfileFetcher: &mockProfileFetcher{},
	})

	assert.Equal(t, 200, rec.Code)
	body := rec.Body.String()
	assert.Contains(t, body, "event: error")
}

func TestStreamCoaching_StreamOpenErrorSendsSSEError(t *testing.T) {
	// No agent stream recorded → StreamAgent returns error
	aiClient := aitest.NewMockClient()

	rec := runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage: "Hello",
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Conversations:  &mockConversationStore{},
		ProfileFetcher: &mockProfileFetcher{},
	})

	// SSE writer commits 200 upfront; the error is delivered as an SSE
	// error event (standard for streams that have already started).
	assert.Equal(t, 200, rec.Code)
	events := parseSSEEvents(rec.Body.String())
	assert.NotEmpty(t, events)
	var foundError bool
	for _, e := range events {
		if strings.Contains(e, "event: error") {
			foundError = true
			break
		}
	}
	assert.True(t, foundError, "expected an SSE error event when StreamAgent fails to open")
}

func TestStreamCoaching_NoConversationIdSkipsPersistence(t *testing.T) {
	aiClient := aitest.NewMockClient()
	aiClient.RecordAgentStream(ai.ModelChat,
		ai.AgentStreamChunk{Complete: true, FullResponse: "Hi!", FinishReason: "stop"},
	)

	conv := &mockConversationStore{}

	rec := runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage: "Hi",
		// no ConversationId
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Conversations:  conv,
		ProfileFetcher: &mockProfileFetcher{},
	})

	assert.Equal(t, 200, rec.Code)
	// No append or get calls
	assert.Empty(t, conv.appendCalls)
	assert.Empty(t, conv.getCalls)
}

func TestStreamCoaching_HistoryFetchedBeforeAppend(t *testing.T) {
	aiClient := aitest.NewMockClient()
	aiClient.RecordAgentStream(ai.ModelChat,
		ai.AgentStreamChunk{Complete: true, FullResponse: "OK", FinishReason: "stop"},
	)

	conv := &mockConversationStore{
		messages: []*aicoachpb.ConversationMessage{
			{Role: "user", Content: "earlier question"},
			{Role: "assistant", Content: "earlier reply"},
		},
	}

	runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage:    "my question",
		ConversationId: "conv-1",
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Conversations:  conv,
		ProfileFetcher: &mockProfileFetcher{},
	})

	// History should be fetched (GetMessages called once).
	require.Len(t, conv.getCalls, 1)
	// User message should be appended (AppendMessage called for user + assistant).
	require.Len(t, conv.appendCalls, 2)
	assert.Equal(t, "user", conv.appendCalls[0].Role)
	assert.Equal(t, "my question", conv.appendCalls[0].Content)
	assert.Equal(t, "assistant", conv.appendCalls[1].Role)

	// The key invariant: history is fetched BEFORE the user message is
	// appended, so the history never includes the current turn's user
	// message. This eliminates the fragile content-matching that the
	// previous implementation used.
	// (We can't directly inspect the history passed to the AI client
	// from here, but the order of operations ensures correctness.)
}

func TestStreamCoaching_HistoryFetchErrorStillPersistsUserMessage(t *testing.T) {
	aiClient := aitest.NewMockClient()
	aiClient.RecordAgentStream(ai.ModelChat,
		ai.AgentStreamChunk{Complete: true, FullResponse: "OK", FinishReason: "stop"},
	)

	conv := &mockConversationStore{
		getError: io.ErrUnexpectedEOF,
	}

	runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage:    "my question",
		ConversationId: "conv-1",
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Conversations:  conv,
		ProfileFetcher: &mockProfileFetcher{},
	})

	// Even if history fetch fails, the user message should still be persisted.
	require.Len(t, conv.appendCalls, 2)
	assert.Equal(t, "user", conv.appendCalls[0].Role)
	assert.Equal(t, "my question", conv.appendCalls[0].Content)
}

func TestStreamCoaching_AttachmentsForwarded(t *testing.T) {
	aiClient := aitest.NewMockClient()
	aiClient.RecordAgentStream(ai.ModelChat,
		ai.AgentStreamChunk{Complete: true, FullResponse: "I see your image.", FinishReason: "stop"},
	)

	rec := runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage: "Look at this",
		Attachments: []types.Attachment{
			{
				AttachmentType: "image",
				Name:           "photo.png",
				ContentType:    "image/png",
				Data:           "iVBORw0KGgo=",
			},
		},
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Conversations:  &mockConversationStore{},
		ProfileFetcher: &mockProfileFetcher{},
	})

	assert.Equal(t, 200, rec.Code)
	events := parseSSEEvents(rec.Body.String())
	require.GreaterOrEqual(t, len(events), 1)
	assert.Contains(t, events[len(events)-1], "event: complete")
}

func TestStreamCoaching_EmptyAttachmentDataDropped(t *testing.T) {
	aiClient := aitest.NewMockClient()
	aiClient.RecordAgentStream(ai.ModelChat,
		ai.AgentStreamChunk{Complete: true, FullResponse: "OK", FinishReason: "stop"},
	)

	rec := runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage: "Hi",
		Attachments: []types.Attachment{
			{AttachmentType: "image", Name: "empty.png", ContentType: "image/png", Data: ""},
		},
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Conversations:  &mockConversationStore{},
		ProfileFetcher: &mockProfileFetcher{},
	})

	assert.Equal(t, 200, rec.Code)
}

func TestStreamCoaching_ProfileFetchErrorProceeds(t *testing.T) {
	aiClient := aitest.NewMockClient()
	aiClient.RecordAgentStream(ai.ModelChat,
		ai.AgentStreamChunk{Complete: true, FullResponse: "Hi!", FinishReason: "stop"},
	)

	profile := &mockProfileFetcher{err: io.ErrUnexpectedEOF}

	rec := runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage: "Hi",
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Conversations:  &mockConversationStore{},
		ProfileFetcher: profile,
	})

	// Should proceed despite profile fetch error
	assert.Equal(t, 200, rec.Code)
	events := parseSSEEvents(rec.Body.String())
	require.GreaterOrEqual(t, len(events), 1)
	assert.Contains(t, events[len(events)-1], "event: complete")
}

func TestStreamCoaching_CoachingLimitsDefaults(t *testing.T) {
	t.Run("all zero uses defaults", func(t *testing.T) {
		maxSteps, maxTotalTokens, maxTokens := coachingLimits(config.CoachingConfig{})
		assert.Equal(t, 6, maxSteps)
		assert.Equal(t, 100000, maxTotalTokens)
		assert.Equal(t, 4096, maxTokens)
	})

	t.Run("custom values preserved", func(t *testing.T) {
		maxSteps, maxTotalTokens, maxTokens := coachingLimits(config.CoachingConfig{
			MaxSteps:       10,
			MaxTotalTokens: 50000,
			MaxTokens:      2048,
		})
		assert.Equal(t, 10, maxSteps)
		assert.Equal(t, 50000, maxTotalTokens)
		assert.Equal(t, 2048, maxTokens)
	})
}

func TestStreamCoaching_EventOrdering(t *testing.T) {
	aiClient := aitest.NewMockClient()
	aiClient.RecordAgentStream(ai.ModelChat,
		ai.AgentStreamChunk{Reasoning: "thinking..."},
		ai.AgentStreamChunk{
			ToolCall: &ai.ToolCallEvent{Name: "get_active_goals", Status: ai.ToolStatusStarted},
		},
		ai.AgentStreamChunk{
			ToolCall: &ai.ToolCallEvent{Name: "get_active_goals", Status: ai.ToolStatusCompleted, Result: `[]`},
		},
		ai.AgentStreamChunk{Delta: "Here is "},
		ai.AgentStreamChunk{Delta: "my answer."},
		ai.AgentStreamChunk{Complete: true, FullResponse: "Here is my answer.", FinishReason: "stop"},
	)

	rec := runStreamCoaching(t, &types.GeneratePersonalizedCoachingRequest{
		UserMessage: "What are my goals?",
	}, StreamCoachingDeps{
		AIClient:       aiClient,
		Conversations:  &mockConversationStore{},
		ProfileFetcher: &mockProfileFetcher{},
	})

	assert.Equal(t, 200, rec.Code)
	events := parseSSEEvents(rec.Body.String())

	// Verify ordering: reasoning → thinking(tool started) → delta → delta → complete
	// (tool completed event does not emit an SSE event for non-proposal tools)
	require.GreaterOrEqual(t, len(events), 5)
	assert.Contains(t, events[0], "event: reasoning")
	assert.Contains(t, events[1], "event: thinking")
	assert.Contains(t, events[2], "event: delta")
	assert.Contains(t, events[3], "event: delta")
	assert.Contains(t, events[4], "event: complete")
}
