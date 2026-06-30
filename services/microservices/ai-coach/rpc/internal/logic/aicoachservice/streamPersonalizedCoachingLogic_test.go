package aicoachservicelogic

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/aitest"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/prompts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"
)

// mockCoachingStream implements aicoach.AICoachService_StreamPersonalizedCoachingServer
// (grpc.ServerStreamingServer[aicoach.PersonalizedCoachingStreamChunk]) for tests.
type mockCoachingStream struct {
	ctx     context.Context
	sent    []*aicoach.PersonalizedCoachingStreamChunk
	sendErr error
}

func (m *mockCoachingStream) Send(chunk *aicoach.PersonalizedCoachingStreamChunk) error {
	if m.sendErr != nil {
		return m.sendErr
	}
	m.sent = append(m.sent, chunk)
	return nil
}

func (m *mockCoachingStream) SetHeader(metadata.MD) error  { return nil }
func (m *mockCoachingStream) SendHeader(metadata.MD) error { return nil }
func (m *mockCoachingStream) SetTrailer(metadata.MD)       {}
func (m *mockCoachingStream) Context() context.Context     { return m.ctx }
func (m *mockCoachingStream) SendMsg(any) error            { return nil }
func (m *mockCoachingStream) RecvMsg(any) error            { return nil }

// newCoachingSvcCtx builds a ServiceContext wired with a mock AI client and a
// real LLMClassifier backed by that mock.
func newCoachingSvcCtx(t *testing.T, mc *aitest.MockClient) *svc.ServiceContext {
	t.Helper()
	return &svc.ServiceContext{
		AIClient:   mc,
		Classifier: safety.NewLLMClassifier(mc),
	}
}

// TestStreamPersonalizedCoaching_CrisisBlocked verifies that a crisis verdict
// short-circuits the model: the deterministic CrisisResponse is streamed and
// the chat model is never called.
func TestStreamPersonalizedCoaching_CrisisBlocked(t *testing.T) {
	mc := aitest.NewMockClient()
	// Classifier returns "crisis" for the user message.
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `{"category":"crisis","confidence":0.9,"reason":"user expresses distress"}`,
	}, ai.Usage{}, 0)
	// Deliberately record NO stream for ModelChat — if the logic calls it,
	// Stream() returns an error and the test fails below.

	svcCtx := newCoachingSvcCtx(t, mc)
	logic := NewStreamPersonalizedCoachingLogic(context.Background(), svcCtx)

	stream := &mockCoachingStream{ctx: context.Background()}
	err := logic.StreamPersonalizedCoaching(&aicoach.PersonalizedCoachingRequest{
		UserId:      "user-1",
		UserMessage: "I can't take this anymore",
	}, stream)
	require.NoError(t, err)

	// The model must never have been called for chat.
	// (MockClient.Stream panics/errs with "no recorded stream" if invoked; the
	// logic would have returned a fallback, not CrisisResponse.)

	require.Len(t, stream.sent, 2, "expected delta + complete chunks")
	assert.Equal(t, prompts.CrisisResponse, stream.sent[0].Delta)
	assert.True(t, stream.sent[1].Complete)
	assert.Equal(t, prompts.CrisisResponse, stream.sent[1].FullResponse)
}

// TestStreamPersonalizedCoaching_SelfHarmBlocked verifies self_harm is treated
// the same as crisis.
func TestStreamPersonalizedCoaching_SelfHarmBlocked(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `{"category":"self_harm","confidence":0.92,"reason":"self-harm ideation"}`,
	}, ai.Usage{}, 0)

	svcCtx := newCoachingSvcCtx(t, mc)
	logic := NewStreamPersonalizedCoachingLogic(context.Background(), svcCtx)

	stream := &mockCoachingStream{ctx: context.Background()}
	err := logic.StreamPersonalizedCoaching(&aicoach.PersonalizedCoachingRequest{
		UserId:      "user-1",
		UserMessage: "I want to hurt myself",
	}, stream)
	require.NoError(t, err)

	require.Len(t, stream.sent, 2)
	assert.Equal(t, prompts.CrisisResponse, stream.sent[0].Delta)
	assert.Equal(t, prompts.CrisisResponse, stream.sent[1].FullResponse)
	assert.True(t, stream.sent[1].Complete)
}

// TestStreamPersonalizedCoaching_SafeProceeds verifies a safe verdict lets the
// request reach the model and the streamed response is relayed.
func TestStreamPersonalizedCoaching_SafeProceeds(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `{"category":"safe","confidence":0.99,"reason":"no concern"}`,
	}, ai.Usage{}, 0)
	mc.RecordStream(ai.ModelChat,
		ai.Chunk{Delta: "Keep up the great work!"},
		ai.Chunk{FinishReason: "stop"},
	)

	svcCtx := newCoachingSvcCtx(t, mc)
	logic := NewStreamPersonalizedCoachingLogic(context.Background(), svcCtx)

	stream := &mockCoachingStream{ctx: context.Background()}
	err := logic.StreamPersonalizedCoaching(&aicoach.PersonalizedCoachingRequest{
		UserId:      "user-1",
		UserMessage: "I had a productive day",
	}, stream)
	require.NoError(t, err)

	// At least one delta and a final complete chunk.
	require.NotEmpty(t, stream.sent)
	last := stream.sent[len(stream.sent)-1]
	assert.True(t, last.Complete)
	assert.Contains(t, last.FullResponse, "Keep up the great work!")
}

// TestStreamPersonalizedCoaching_ClassifierErrorFailsOpen verifies that when
// the classifier itself errors, the request proceeds to the model (fail-open).
func TestStreamPersonalizedCoaching_ClassifierErrorFailsOpen(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.SetError(ai.ErrModelUnavailable)
	// The classifier call will fail; the logic should proceed. But the mock's
	// error applies to ALL calls including Stream, so the logic will hit the
	// stream-open fallback. We assert it does NOT return CrisisResponse and
	// does not error out.
	svcCtx := newCoachingSvcCtx(t, mc)
	logic := NewStreamPersonalizedCoachingLogic(context.Background(), svcCtx)

	stream := &mockCoachingStream{ctx: context.Background()}
	err := logic.StreamPersonalizedCoaching(&aicoach.PersonalizedCoachingRequest{
		UserId:      "user-1",
		UserMessage: "help me stay on track",
	}, stream)
	require.NoError(t, err)

	require.NotEmpty(t, stream.sent)
	last := stream.sent[len(stream.sent)-1]
	assert.True(t, last.Complete)
	// Fallback message, NOT the crisis response — proves we failed open rather
	// than blocking.
	assert.NotEqual(t, prompts.CrisisResponse, last.FullResponse)
}
