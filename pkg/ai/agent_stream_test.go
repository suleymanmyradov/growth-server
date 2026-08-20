package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
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

	// Should have two tool call events for one tool call: started + completed.
	require.Len(t, toolCalls, 2)
	assert.Equal(t, "echo", toolCalls[0].Name)
	assert.Equal(t, ToolStatusStarted, toolCalls[0].Status)
	assert.Empty(t, toolCalls[0].Result)
	assert.Equal(t, "echo", toolCalls[1].Name)
	assert.Equal(t, ToolStatusCompleted, toolCalls[1].Status)
	assert.Contains(t, toolCalls[1].Result, "hello")

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

// TestClient_StreamAgent_FallbackOnStreamOpenError reproduces the production
// 503-on-stream-open scenario: the primary model returns 503 ("high demand")
// when the agentic loop tries to open the stream. Without fallback, this kills
// the whole turn. With FallbackProviders configured, the loop must re-bind
// tools to the fallback model and complete the turn via the fallback.
func TestClient_StreamAgent_FallbackOnStreamOpenError(t *testing.T) {
	var (
		primaryCalls   atomic.Int64
		fallbackCalls  atomic.Int64
		fallbackAuth   string
		fallbackModel  string
		fallbackStream bool
	)

	// Primary server: always returns 503 (Google "high demand").
	primaryServer := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		primaryCalls.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"code":503,"message":"This model is currently experiencing high demand","type":"server_error","status":"UNAVAILABLE"}}`))
	})
	defer primaryServer.Close()

	// Fallback server: streams a direct answer (no tool calls).
	fallbackServer := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalls.Add(1)
		fallbackAuth = r.Header.Get("Authorization")
		fallbackStream = r.Header.Get("Accept") == "text/event-stream" || strings.Contains(r.URL.Path, "completions")

		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		fallbackModel, _ = body["model"].(string)

		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []map[string]any{
			{"id": "chatcmpl-fb1", "object": "chat.completion.chunk", "model": fallbackModel,
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "fallback "}, "finish_reason": nil}}},
			{"id": "chatcmpl-fb2", "object": "chat.completion.chunk", "model": fallbackModel,
				"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "answer"}, "finish_reason": "stop"}},
				"usage":   map[string]any{"prompt_tokens": 8, "completion_tokens": 4, "total_tokens": 12}},
		}
		for _, chunk := range chunks {
			data, _ := json.Marshal(chunk)
			_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})
	defer fallbackServer.Close()

	cfg := testConfig("primary-key")
	cfg.BaseURL = primaryServer.URL
	cfg.MaxRetries = 0 // Don't retry the primary; go straight to fallback.
	cfg.FallbackPolicy = FallbackPolicy{Enabled: true, MaxFailures: 1}
	cfg.FallbackProviders = []ProviderConfig{{
		APIKey:  "google-key",
		BaseURL: fallbackServer.URL,
		Models: map[string]string{
			string(ModelChat):     "gemini-flash-latest",
			string(ModelFallback): "gemini-flash-latest",
		},
	}}

	c, err := New(cfg, WithHTTPClient(primaryServer.Client()))
	require.NoError(t, err)

	sr, err := c.StreamAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "Hello coach"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     5,
	})
	require.NoError(t, err)
	defer sr.Close()

	var deltas strings.Builder
	var gotComplete bool
	var completeFull string
	for {
		chunk, recvErr := sr.Recv()
		if recvErr == io.EOF {
			break
		}
		require.NoError(t, recvErr)
		if chunk.Delta != "" {
			deltas.WriteString(chunk.Delta)
		}
		if chunk.Complete {
			gotComplete = true
			completeFull = chunk.FullResponse
		}
	}

	// The turn must complete via the fallback, not die on the primary 503.
	assert.True(t, gotComplete, "should receive a Complete event via fallback")
	assert.Equal(t, "fallback answer", deltas.String())
	assert.Equal(t, "fallback answer", completeFull)

	// Primary was attempted and failed; fallback served the turn.
	assert.GreaterOrEqual(t, primaryCalls.Load(), int64(1), "primary should be attempted")
	assert.GreaterOrEqual(t, fallbackCalls.Load(), int64(1), "fallback should serve the turn")
	assert.Equal(t, "Bearer google-key", fallbackAuth, "fallback should use the fallback provider's API key")
	assert.Equal(t, "gemini-flash-latest", fallbackModel, "fallback should use the fallback provider's model")
	_ = fallbackStream
}

// TestClient_StreamAgent_FallbackAllFail verifies that when both the primary
// and every fallback model fail to open the stream, the agent emits an Error
// event (rather than hanging or panicking).
func TestClient_StreamAgent_FallbackAllFail(t *testing.T) {
	primaryServer := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"code":503,"message":"primary overloaded"}}`))
	})
	defer primaryServer.Close()

	fallbackServer := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"code":429,"message":"fallback rate limited"}}`))
	})
	defer fallbackServer.Close()

	cfg := testConfig("primary-key")
	cfg.BaseURL = primaryServer.URL
	cfg.MaxRetries = 0
	cfg.FallbackPolicy = FallbackPolicy{Enabled: true, MaxFailures: 1}
	cfg.FallbackProviders = []ProviderConfig{{
		APIKey:  "fb-key",
		BaseURL: fallbackServer.URL,
		Models:  map[string]string{string(ModelChat): "gemini-flash-latest"},
	}}

	c, err := New(cfg, WithHTTPClient(primaryServer.Client()))
	require.NoError(t, err)

	sr, err := c.StreamAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     5,
	})
	require.NoError(t, err)
	defer sr.Close()

	var gotError bool
	for {
		chunk, recvErr := sr.Recv()
		if recvErr == io.EOF {
			break
		}
		if chunk.Error != nil || recvErr != nil {
			gotError = true
			break
		}
	}
	assert.True(t, gotError, "should emit an error event when primary and all fallbacks fail")
}

// TestClient_StreamAgent_IncompleteFinishContinues verifies that a provider
// response cut short mid-answer gets one bounded continuation attempt. The
// caller receives a single completed response made from both streamed parts.
func TestClient_StreamAgent_IncompleteFinishContinues(t *testing.T) {
	for _, finishReason := range []any{"length", nil} {
		t.Run(fmt.Sprintf("finish_reason_%v", finishReason), func(t *testing.T) {
			callCount := 0
			server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				w.Header().Set("Content-Type", "text/event-stream")
				chunks := []map[string]any{}
				if callCount == 1 {
					chunks = []map[string]any{
						{"id": "chatcmpl-1", "object": "chat.completion.chunk", "model": "openai/gpt-4o-mini",
							"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "The most important "}, "finish_reason": nil}}},
						{"id": "chatcmpl-2", "object": "chat.completion.chunk", "model": "openai/gpt-4o-mini",
							"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "step is to rest."}, "finish_reason": finishReason}},
							"usage":   map[string]any{"prompt_tokens": 100, "completion_tokens": 157, "total_tokens": 257}},
					}
				} else {
					chunks = []map[string]any{
						{"id": "chatcmpl-3", "object": "chat.completion.chunk", "model": "openai/gpt-4o-mini",
							"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": " Take a five-minute pause now."}, "finish_reason": "stop"}},
							"usage":   map[string]any{"prompt_tokens": 120, "completion_tokens": 12, "total_tokens": 132}},
					}
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
				Messages:     []Message{{Role: RoleUser, Content: "Give me a long coaching response"}},
				Tools:        []Tool{EchoTool},
				MaxSteps:     5,
			})
			require.NoError(t, err)
			defer sr.Close()

			var deltas strings.Builder
			var completeChunk *AgentStreamChunk
			for {
				chunk, recvErr := sr.Recv()
				if recvErr == io.EOF {
					break
				}
				require.NoError(t, recvErr)
				if chunk.Delta != "" {
					deltas.WriteString(chunk.Delta)
				}
				if chunk.Complete {
					completeChunk = &chunk
				}
			}

			assert.Equal(t, "The most important step is to rest. Take a five-minute pause now.", deltas.String())
			require.NotNil(t, completeChunk)
			assert.Equal(t, deltas.String(), completeChunk.FullResponse)
			assert.Equal(t, 2, callCount)
		})
	}
}

func TestClient_StreamAgent_IncompleteContinuationReturnsError(t *testing.T) {
	callCount := 0
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/event-stream")
		chunk := map[string]any{
			"id": "chatcmpl-incomplete", "object": "chat.completion.chunk", "model": "openai/gpt-4o-mini",
			"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "partial"}, "finish_reason": "length"}},
		}
		data, _ := json.Marshal(chunk)
		_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	sr, err := c.StreamAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "Give me a long coaching response"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     5,
	})
	require.NoError(t, err)
	defer sr.Close()

	var complete bool
	var streamErr error
	for {
		chunk, recvErr := sr.Recv()
		if recvErr == io.EOF {
			break
		}
		if chunk.Complete {
			complete = true
		}
		if recvErr != nil {
			streamErr = recvErr
			break
		}
	}

	assert.Equal(t, 2, callCount)
	assert.False(t, complete)
	assert.ErrorIs(t, streamErr, ErrStreamIncomplete)
}

// TestClient_StreamAgent_MaxTokensPassedToRequest verifies that when
// MaxTokens is set on the AgentRequest, the value is sent to the provider
// in the streaming request body (preventing Gemini's low internal default
// from truncating the output).
func TestClient_StreamAgent_MaxTokensPassedToRequest(t *testing.T) {
	var capturedMaxTokens any
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		capturedMaxTokens = body["max_tokens"]

		w.Header().Set("Content-Type", "text/event-stream")
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

	maxTok := 4096
	sr, err := c.StreamAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     5,
		MaxTokens:    &maxTok,
	})
	require.NoError(t, err)
	defer sr.Close()

	for {
		_, recvErr := sr.Recv()
		if recvErr == io.EOF {
			break
		}
		require.NoError(t, recvErr)
	}

	// The provider must receive max_tokens in the request body so Gemini
	// doesn't apply its low internal default.
	assert.NotNil(t, capturedMaxTokens, "max_tokens must be sent to the provider")
	if mt, ok := capturedMaxTokens.(float64); ok {
		assert.Equal(t, float64(4096), mt)
	}
}
