package aitest

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/pkg/ai"
)

func TestMockClient_Generate(t *testing.T) {
	mc := NewMockClient()
	mc.RecordResponse(ai.ModelCheap, ai.Message{
		Role:    ai.RoleAssistant,
		Content: "Great work on your check-in!",
	}, ai.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}, 0.0001)

	resp, err := mc.Generate(context.Background(), ai.GenerateRequest{
		ModelProfile: ai.ModelCheap,
		Messages:     []ai.Message{{Role: ai.RoleUser, Content: "How did I do?"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Great work on your check-in!", resp.Message.Content)
	assert.Equal(t, 15, resp.Usage.TotalTokens)
}

func TestMockClient_Generate_NoRecordedResponse(t *testing.T) {
	mc := NewMockClient()
	_, err := mc.Generate(context.Background(), ai.GenerateRequest{
		ModelProfile: ai.ModelChat,
		Messages:     []ai.Message{{Role: ai.RoleUser, Content: "Hello"}},
	})
	assert.Error(t, err)
}

func TestMockClient_SetError(t *testing.T) {
	mc := NewMockClient()
	mc.SetError(ai.ErrModelUnavailable)

	_, err := mc.Generate(context.Background(), ai.GenerateRequest{
		ModelProfile: ai.ModelCheap,
		Messages:     []ai.Message{{Role: ai.RoleUser, Content: "test"}},
	})
	assert.ErrorIs(t, err, ai.ErrModelUnavailable)
}

func TestMockClient_Calls(t *testing.T) {
	mc := NewMockClient()
	mc.RecordResponse(ai.ModelCheap, ai.Message{Role: ai.RoleAssistant, Content: "hi"}, ai.Usage{}, 0)

	_, _ = mc.Generate(context.Background(), ai.GenerateRequest{
		ModelProfile: ai.ModelCheap,
		Messages:     []ai.Message{{Role: ai.RoleUser, Content: "hello"}},
	})

	calls := mc.Calls()
	assert.Len(t, calls, 1)
	assert.Equal(t, ai.ModelCheap, calls[0].ModelProfile)
}

func TestMockClient_Stream(t *testing.T) {
	mc := NewMockClient()
	mc.RecordStream(ai.ModelCheap,
		ai.Chunk{Delta: "Hel"},
		ai.Chunk{Delta: "lo!"},
	)

	sr, err := mc.Stream(context.Background(), ai.GenerateRequest{
		ModelProfile: ai.ModelCheap,
		Messages:     []ai.Message{{Role: ai.RoleUser, Content: "hi"}},
	})
	require.NoError(t, err)
	defer sr.Close()

	var collected string
	for {
		chunk, err := sr.Recv()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		collected += chunk.Delta
	}
	assert.Equal(t, "Hello!", collected)
}

func TestMockClient_RunAgent(t *testing.T) {
	mc := NewMockClient()
	mc.RecordResponse(ai.ModelChat, ai.Message{
		Role:    ai.RoleAssistant,
		Content: "I used the tool and got the result.",
	}, ai.Usage{PromptTokens: 20, CompletionTokens: 10, TotalTokens: 30}, 0.001)

	resp, err := mc.RunAgent(context.Background(), ai.AgentRequest{
		ModelProfile: ai.ModelChat,
		Messages:     []ai.Message{{Role: ai.RoleUser, Content: "Use the tool"}},
		Tools:        []ai.Tool{}, // mock doesn't actually call tools
		MaxSteps:     5,
	})
	require.NoError(t, err)
	assert.Equal(t, 1, resp.Steps)
}
