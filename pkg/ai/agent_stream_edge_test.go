package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sseWrite writes one SSE data line to the response writer.
func sseWrite(w http.ResponseWriter, payload map[string]any) {
	data, _ := json.Marshal(payload)
	_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
}

// sseDone writes the terminating [DONE] marker.
func sseDone(w http.ResponseWriter) {
	_, _ = w.Write([]byte("data: [DONE]\n\n"))
}

// streamToolCallChunk builds an SSE chunk whose delta carries a tool call.
func streamToolCallChunk(id, name, args string) map[string]any {
	return map[string]any{
		"id": "chatcmpl-x", "object": "chat.completion.chunk", "model": "openai/gpt-4o-mini",
		"choices": []any{map[string]any{
			"index": 0,
			"delta": map[string]any{
				"role": "assistant",
				"tool_calls": []any{map[string]any{
					"index": 0,
					"id":    id,
					"type":  "function",
					"function": map[string]any{
						"name":      name,
						"arguments": args,
					},
				}},
			},
			"finish_reason": "tool_calls",
		}},
		"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
	}
}

// streamMultiToolCallChunk builds an SSE chunk whose delta carries several
// tool calls in one assistant message (each with its own index).
func streamMultiToolCallChunk(calls []map[string]string) map[string]any {
	tcs := make([]any, 0, len(calls))
	for i, c := range calls {
		tcs = append(tcs, map[string]any{
			"index": i,
			"id":    c["id"],
			"type":  "function",
			"function": map[string]any{
				"name":      c["name"],
				"arguments": c["arguments"],
			},
		})
	}
	return map[string]any{
		"id": "chatcmpl-x", "object": "chat.completion.chunk", "model": "openai/gpt-4o-mini",
		"choices": []any{map[string]any{
			"index":         0,
			"delta":         map[string]any{"role": "assistant", "tool_calls": tcs},
			"finish_reason": "tool_calls",
		}},
		"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
	}
}

// streamContentChunks builds SSE chunks that stream a text answer as deltas.
func streamContentChunks(parts ...string) []map[string]any {
	out := make([]map[string]any, 0, len(parts))
	for _, p := range parts {
		out = append(out, map[string]any{
			"id": "chatcmpl-y", "object": "chat.completion.chunk", "model": "openai/gpt-4o-mini",
			"choices": []any{map[string]any{
				"index":         0,
				"delta":         map[string]any{"content": p},
				"finish_reason": nil,
			}},
		})
	}
	// Final chunk carries usage and finish_reason=stop.
	out = append(out, map[string]any{
		"id": "chatcmpl-y", "object": "chat.completion.chunk", "model": "openai/gpt-4o-mini",
		"choices": []any{map[string]any{
			"index":         0,
			"delta":         map[string]any{},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{"prompt_tokens": 20, "completion_tokens": 10, "total_tokens": 30},
	})
	return out
}

// drainStream reads all chunks from an agent stream reader, collecting
// deltas, tool-call events, and the complete chunk. Returns an error if
// the stream ends with an Error chunk (mirroring Recv's behavior).
func drainStream(t *testing.T, sr AgentStreamReader) (deltas string, toolCalls []*ToolCallEvent, complete *AgentStreamChunk, streamErr error) {
	t.Helper()
	for {
		chunk, recvErr := sr.Recv()
		if recvErr == io.EOF {
			return
		}
		if recvErr != nil {
			streamErr = recvErr
			return
		}
		if chunk.ToolCall != nil {
			toolCalls = append(toolCalls, chunk.ToolCall)
		}
		if chunk.Delta != "" {
			deltas += chunk.Delta
		}
		if chunk.Complete {
			cc := chunk
			complete = &cc
		}
		if chunk.Error != nil {
			streamErr = chunk.Error
			return
		}
	}
}

// TestClient_StreamAgent_UnknownTool verifies that an unknown tool call is
// surfaced as a ToolCall event with an Error field, and the loop continues
// to a final answer.
func TestClient_StreamAgent_UnknownTool(t *testing.T) {
	callCount := 0
	server := mockOpenRouterServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		callCount++
		if callCount == 1 {
			sseWrite(w, streamToolCallChunk("call_1", "does_not_exist", `{}`))
			sseDone(w)
			return
		}
		for _, c := range streamContentChunks("recovered") {
			sseWrite(w, c)
		}
		sseDone(w)
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	sr, err := c.StreamAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "call missing tool"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     5,
	})
	require.NoError(t, err)
	defer sr.Close()

	deltas, toolCalls, complete, streamErr := drainStream(t, sr)
	require.NoError(t, streamErr)
	require.Len(t, toolCalls, 2, "started + completed events")
	assert.Equal(t, "does_not_exist", toolCalls[0].Name)
	assert.Equal(t, ToolStatusStarted, toolCalls[0].Status)
	assert.Equal(t, "does_not_exist", toolCalls[1].Name)
	assert.Equal(t, ToolStatusCompleted, toolCalls[1].Status)
	assert.Contains(t, toolCalls[1].Error, "unknown tool")
	assert.Equal(t, "recovered", deltas)
	require.NotNil(t, complete)
	assert.Equal(t, "recovered", complete.FullResponse)
}

// TestClient_StreamAgent_ToolExecutionError verifies that a tool returning
// an error is surfaced as a ToolCall event with the error message, and the
// loop continues to a final answer.
func TestClient_StreamAgent_ToolExecutionError(t *testing.T) {
	callCount := 0
	server := mockOpenRouterServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		callCount++
		if callCount == 1 {
			sseWrite(w, streamToolCallChunk("call_1", "failing_tool", `{"text":"x"}`))
			sseDone(w)
			return
		}
		for _, c := range streamContentChunks("handled") {
			sseWrite(w, c)
		}
		sseDone(w)
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	sr, err := c.StreamAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "call failing tool"}},
		Tools:        []Tool{failingTool},
		MaxSteps:     5,
	})
	require.NoError(t, err)
	defer sr.Close()

	_, toolCalls, complete, streamErr := drainStream(t, sr)
	require.NoError(t, streamErr)
	require.Len(t, toolCalls, 2, "started + completed events")
	assert.Equal(t, "failing_tool", toolCalls[0].Name)
	assert.Equal(t, ToolStatusStarted, toolCalls[0].Status)
	assert.Equal(t, "failing_tool", toolCalls[1].Name)
	assert.Equal(t, ToolStatusCompleted, toolCalls[1].Status)
	assert.Contains(t, toolCalls[1].Error, "boom")
	require.NotNil(t, complete)
	assert.Equal(t, "handled", complete.FullResponse)
}

// TestClient_StreamAgent_MaxSteps verifies that when the model keeps
// requesting tool calls, the stream ends with an Error event wrapping
// ErrMaxSteps (StreamAgent, unlike RunAgent, surfaces this as an error).
func TestClient_StreamAgent_MaxSteps(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Always stream a tool call; never a final answer.
		sseWrite(w, streamToolCallChunk("call_x", "echo", `{"text":"again"}`))
		sseDone(w)
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	sr, err := c.StreamAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "loop forever"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     2,
	})
	require.NoError(t, err)
	defer sr.Close()

	_, _, _, streamErr := drainStream(t, sr)
	require.Error(t, streamErr)
	assert.ErrorIs(t, streamErr, ErrMaxSteps)
}

// TestClient_StreamAgent_MaxTotalTokens verifies that exceeding the
// cumulative token budget mid-stream surfaces an Error event wrapping
// ErrMaxTokens (not ErrMaxSteps) with the "max total tokens exceeded" message.
func TestClient_StreamAgent_MaxTotalTokens(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		// Single step whose usage already exceeds the small budget.
		chunk := streamToolCallChunk("call_1", "echo", `{"text":"hi"}`)
		// Override usage to exceed the budget.
		chunk["usage"] = map[string]any{"prompt_tokens": 50, "completion_tokens": 60, "total_tokens": 110}
		sseWrite(w, chunk)
		sseDone(w)
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	sr, err := c.StreamAgent(context.Background(), AgentRequest{
		ModelProfile:   ModelChat,
		Messages:       []Message{{Role: RoleUser, Content: "blow the budget"}},
		Tools:          []Tool{EchoTool},
		MaxSteps:       5,
		MaxTotalTokens: 100,
	})
	require.NoError(t, err)
	defer sr.Close()

	_, _, _, streamErr := drainStream(t, sr)
	require.Error(t, streamErr)
	assert.ErrorIs(t, streamErr, ErrMaxTokens)
	assert.Contains(t, streamErr.Error(), "max total tokens exceeded")
}

// TestClient_StreamAgent_MultipleToolCallsInOneStep verifies that when the
// model requests several tool calls in a single streamed assistant message,
// each is executed and surfaced as its own ToolCall event.
func TestClient_StreamAgent_MultipleToolCallsInOneStep(t *testing.T) {
	callCount := 0
	server := mockOpenRouterServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		callCount++
		if callCount == 1 {
			sseWrite(w, streamMultiToolCallChunk([]map[string]string{
				{"id": "call_a", "name": "echo", "arguments": `{"text":"first"}`},
				{"id": "call_b", "name": "echo", "arguments": `{"text":"second"}`},
			}))
			sseDone(w)
			return
		}
		for _, c := range streamContentChunks("got both") {
			sseWrite(w, c)
		}
		sseDone(w)
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	sr, err := c.StreamAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "echo twice"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     5,
	})
	require.NoError(t, err)
	defer sr.Close()

	deltas, toolCalls, complete, streamErr := drainStream(t, sr)
	require.NoError(t, streamErr)
	require.Len(t, toolCalls, 4, "two tool calls × (started + completed)")
	// First tool: started then completed
	assert.Equal(t, "echo", toolCalls[0].Name)
	assert.Equal(t, ToolStatusStarted, toolCalls[0].Status)
	assert.Equal(t, "echo", toolCalls[1].Name)
	assert.Equal(t, ToolStatusCompleted, toolCalls[1].Status)
	assert.Contains(t, toolCalls[1].Result, "first")
	// Second tool: started then completed
	assert.Equal(t, "echo", toolCalls[2].Name)
	assert.Equal(t, ToolStatusStarted, toolCalls[2].Status)
	assert.Equal(t, "echo", toolCalls[3].Name)
	assert.Equal(t, ToolStatusCompleted, toolCalls[3].Status)
	assert.Contains(t, toolCalls[3].Result, "second")
	assert.Equal(t, "got both", deltas)
	require.NotNil(t, complete)
	assert.Equal(t, "got both", complete.FullResponse)
	assert.Equal(t, 2, callCount)
}

// TestClient_StreamAgent_MultiStepChain verifies a 3-step streaming loop:
// tool call → tool call → final streamed answer.
func TestClient_StreamAgent_MultiStepChain(t *testing.T) {
	callCount := 0
	server := mockOpenRouterServer(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		callCount++
		switch callCount {
		case 1:
			sseWrite(w, streamToolCallChunk("call_1", "echo", `{"text":"step1"}`))
			sseDone(w)
		case 2:
			sseWrite(w, streamToolCallChunk("call_2", "echo", `{"text":"step2"}`))
			sseDone(w)
		default:
			for _, c := range streamContentChunks("final ", "answer") {
				sseWrite(w, c)
			}
			sseDone(w)
		}
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	sr, err := c.StreamAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "two-step chain"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     5,
	})
	require.NoError(t, err)
	defer sr.Close()

	deltas, toolCalls, complete, streamErr := drainStream(t, sr)
	require.NoError(t, streamErr)
	require.Len(t, toolCalls, 4, "two tool calls across two steps × (started + completed)")
	// Step 1 tool: started then completed
	assert.Equal(t, 1, toolCalls[0].Step)
	assert.Equal(t, ToolStatusStarted, toolCalls[0].Status)
	assert.Equal(t, 1, toolCalls[1].Step)
	assert.Equal(t, ToolStatusCompleted, toolCalls[1].Status)
	// Step 2 tool: started then completed
	assert.Equal(t, 2, toolCalls[2].Step)
	assert.Equal(t, ToolStatusStarted, toolCalls[2].Status)
	assert.Equal(t, 2, toolCalls[3].Step)
	assert.Equal(t, ToolStatusCompleted, toolCalls[3].Status)
	assert.Equal(t, "final answer", deltas)
	require.NotNil(t, complete)
	assert.Equal(t, "final answer", complete.FullResponse)
	assert.Equal(t, 3, callCount, "three LLM calls expected")
}
