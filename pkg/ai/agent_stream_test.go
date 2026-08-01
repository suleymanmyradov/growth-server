package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient_StreamAgent_ToolCallThenAnswer verifies the full agent loop:
// step 1 emits a tool call, step 2 emits the final answer with streaming
// deltas.
func TestClient_StreamAgent_ToolCallThenAnswer(t *testing.T) {
	callCount := 0
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/event-stream")

		if callCount == 1 {
			// Step 1: model requests a tool call (streamed).
			chunks := []map[string]any{
				{
					"id":     "chatcmpl-1",
					"object": "chat.completion.chunk",
					"model":  "openai/gpt-4o-mini",
					"choices": []any{map[string]any{
						"index": 0,
						"delta": map[string]any{
							"role": "assistant",
							"tool_calls": []any{map[string]any{
								"index": 0,
								"id":    "call_1",
								"type":  "function",
								"function": map[string]any{
									"name":      "echo",
									"arguments": `{"text":"hello"}`,
								},
							}},
						},
						"finish_reason": "tool_calls",
					}},
					"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
				},
			}
			for _, chunk := range chunks {
				data, _ := json.Marshal(chunk)
				_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
			}
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			return
		}

		// Step 2: model streams the final answer as content deltas.
		chunks := []map[string]any{
			{"id": "chatcmpl-2", "object": "chat.completion.chunk", "model": "openai/gpt-4o-mini",
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "The echo "}, "finish_reason": nil}}},
			{"id": "chatcmpl-3", "object": "chat.completion.chunk", "model": "openai/gpt-4o-mini",
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "returned: hello"}, "finish_reason": "stop"}},
				"usage":   map[string]any{"prompt_tokens": 20, "completion_tokens": 10, "total_tokens": 30}},
		}
		for _, chunk := range chunks {
			data, _ := json.Marshal(chunk)
			_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL

	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	sr, err := c.StreamAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "Use the echo tool"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     5,
	})
	require.NoError(t, err)
	defer sr.Close()

	var deltas strings.Builder
	var toolCalls []*ToolCallEvent
	var completeChunk *AgentStreamChunk

	for {
		chunk, recvErr := sr.Recv()
		if recvErr == io.EOF {
			break
		}
		require.NoError(t, recvErr)

		if chunk.ToolCall != nil {
			toolCalls = append(toolCalls, chunk.ToolCall)
		}
		if chunk.Delta != "" {
			deltas.WriteString(chunk.Delta)
		}
		if chunk.Complete {
			cc := chunk
			completeChunk = &cc
		}
	}

	// Should have one tool call event (step 1).
	require.Len(t, toolCalls, 1)
	assert.Equal(t, "echo", toolCalls[0].Name)
	assert.Equal(t, 1, toolCalls[0].Step)
	assert.Contains(t, toolCalls[0].Result, "hello")

	// Should have streamed the final answer as deltas.
	assert.Equal(t, "The echo returned: hello", deltas.String())

	// Should have a complete chunk with the full response.
	require.NotNil(t, completeChunk)
	assert.Equal(t, "The echo returned: hello", completeChunk.FullResponse)
	require.NotNil(t, completeChunk.Usage)
	assert.Greater(t, completeChunk.Usage.TotalTokens, 0)

	// Two LLM calls: step 1 (tool call) + step 2 (final answer).
	assert.Equal(t, 2, callCount)
}

// TestClient_StreamAgent_DirectAnswer verifies that when the model answers
// directly without tool calls, deltas are streamed and a Complete event is
// emitted.
func TestClient_StreamAgent_DirectAnswer(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []map[string]any{
			{"id": "chatcmpl-1", "object": "chat.completion.chunk", "model": "openai/gpt-4o-mini",
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "Hi "}, "finish_reason": nil}}},
			{"id": "chatcmpl-2", "object": "chat.completion.chunk", "model": "openai/gpt-4o-mini",
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "there!"}, "finish_reason": "stop"}},
				"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 5, "total_tokens": 10}},
		}
		for _, chunk := range chunks {
			data, _ := json.Marshal(chunk)
			_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL

	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	sr, err := c.StreamAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "Say hi"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     5,
	})
	require.NoError(t, err)
	defer sr.Close()

	var deltas strings.Builder
	var toolCalls []*ToolCallEvent
	var gotComplete bool

	for {
		chunk, recvErr := sr.Recv()
		if recvErr == io.EOF {
			break
		}
		require.NoError(t, recvErr)

		if chunk.ToolCall != nil {
			toolCalls = append(toolCalls, chunk.ToolCall)
		}
		if chunk.Delta != "" {
			deltas.WriteString(chunk.Delta)
		}
		if chunk.Complete {
			gotComplete = true
			assert.Equal(t, "Hi there!", chunk.FullResponse)
		}
	}

	assert.Empty(t, toolCalls, "no tool calls expected for direct answer")
	assert.Equal(t, "Hi there!", deltas.String())
	assert.True(t, gotComplete, "should receive a Complete event")
}

// TestClient_StreamAgent_NoTools verifies the error when no tools are provided.
func TestClient_StreamAgent_NoTools(t *testing.T) {
	cfg := testConfig("test-key")
	c, err := New(cfg)
	require.NoError(t, err)

	_, err = c.StreamAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
		Tools:        []Tool{},
		MaxSteps:     5,
	})
	assert.ErrorIs(t, err, ErrNoTools)
}

// TestClient_StreamAgent_Close verifies that Close cancels the loop and
// releases resources without blocking.
func TestClient_StreamAgent_Close(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Send a complete response so the loop finishes naturally.
		chunks := []map[string]any{
			{"id": "chatcmpl-1", "object": "chat.completion.chunk", "model": "openai/gpt-4o-mini",
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "Hi"}, "finish_reason": "stop"}},
				"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 2, "total_tokens": 7}},
		}
		for _, chunk := range chunks {
			data, _ := json.Marshal(chunk)
			_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL

	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	sr, err := c.StreamAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     5,
	})
	require.NoError(t, err)

	// Read until complete, then close.
	for {
		chunk, recvErr := sr.Recv()
		if recvErr == io.EOF {
			break
		}
		require.NoError(t, recvErr)
		if chunk.Complete {
			break
		}
	}

	sr.Close()

	// After Close, Recv should return EOF.
	_, err = sr.Recv()
	assert.Equal(t, io.EOF, err)
}
