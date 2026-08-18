package ai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingTool is a test tool that always returns an error, used to verify
// the agent loop surfaces tool execution errors as tool-result messages
// and continues the loop instead of aborting.
var failingTool = NewTool[EchoInput, EchoOutput](ToolSpec{
	Name:        "failing_tool",
	Description: "Always returns an error.",
	Handler: func(_ context.Context, _ EchoInput) (EchoOutput, error) {
		return EchoOutput{}, errors.New("boom")
	},
})

// toolCallChoice builds an OpenAI chat.completion choice that requests a
// single tool call.
func toolCallChoice(id, name, args string) map[string]any {
	return map[string]any{
		"index": 0,
		"message": map[string]any{
			"role": "assistant",
			"tool_calls": []any{map[string]any{
				"id":   id,
				"type": "function",
				"function": map[string]any{
					"name":      name,
					"arguments": args,
				},
			}},
		},
		"finish_reason": "tool_calls",
	}
}

// multiToolCallChoice builds an OpenAI chat.completion choice that
// requests several tool calls in one assistant message.
func multiToolCallChoice(calls []map[string]string) map[string]any {
	tcs := make([]any, 0, len(calls))
	for i, c := range calls {
		tcs = append(tcs, map[string]any{
			"id":   c["id"],
			"type": "function",
			"function": map[string]any{
				"name":      c["name"],
				"arguments": c["arguments"],
			},
			"index": i,
		})
	}
	return map[string]any{
		"index": 0,
		"message": map[string]any{
			"role":       "assistant",
			"tool_calls": tcs,
		},
		"finish_reason": "tool_calls",
	}
}

// textChoice builds an OpenAI chat.completion choice with a plain text
// assistant message (the final answer).
func textChoice(content string) map[string]any {
	return map[string]any{
		"index": 0,
		"message": map[string]any{
			"role":    "assistant",
			"content": content,
		},
		"finish_reason": "stop",
	}
}

// completionResponse assembles a full chat.completion payload.
func completionResponse(id string, choice map[string]any, usage map[string]any) map[string]any {
	return map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"model":   "openai/gpt-4o-mini",
		"choices": []any{choice},
		"usage":   usage,
	}
}

// TestClient_RunAgent_UnknownTool verifies that when the model calls a tool
// that does not exist in the tool map, the loop records an error tool-result
// message and continues to a final answer rather than aborting.
func TestClient_RunAgent_UnknownTool(t *testing.T) {
	callCount := 0
	server := mockOpenRouterServer(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			_ = json.NewEncoder(w).Encode(completionResponse("c1",
				toolCallChoice("call_1", "does_not_exist", `{}`),
				map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}))
			return
		}
		_ = json.NewEncoder(w).Encode(completionResponse("c2",
			textChoice("recovered from unknown tool"),
			map[string]any{"prompt_tokens": 20, "completion_tokens": 10, "total_tokens": 30}))
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	resp, err := c.RunAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "call a missing tool"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     5,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Steps)
	// The final answer should be present.
	assert.Contains(t, resp.Messages[len(resp.Messages)-1].Content, "recovered from unknown tool")
	// One of the intermediate messages is a tool result reporting the
	// unknown tool.
	foundUnknown := false
	for _, m := range resp.Messages {
		if m.Role == RoleTool && strings.Contains(m.Content, "unknown tool") {
			foundUnknown = true
		}
	}
	assert.True(t, foundUnknown, "expected a tool-result message reporting the unknown tool")
}

// TestClient_RunAgent_ToolExecutionError verifies that a tool returning an
// error is surfaced as a tool-result message with an error payload and the
// loop continues to a final answer.
func TestClient_RunAgent_ToolExecutionError(t *testing.T) {
	callCount := 0
	server := mockOpenRouterServer(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			_ = json.NewEncoder(w).Encode(completionResponse("c1",
				toolCallChoice("call_1", "failing_tool", `{"text":"x"}`),
				map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}))
			return
		}
		_ = json.NewEncoder(w).Encode(completionResponse("c2",
			textChoice("handled tool error"),
			map[string]any{"prompt_tokens": 20, "completion_tokens": 10, "total_tokens": 30}))
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	resp, err := c.RunAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "call failing tool"}},
		Tools:        []Tool{failingTool},
		MaxSteps:     5,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Steps)
	assert.Contains(t, resp.Messages[len(resp.Messages)-1].Content, "handled tool error")

	// A tool-result message should carry the error payload.
	foundErr := false
	for _, m := range resp.Messages {
		if m.Role == RoleTool && strings.Contains(m.Content, "boom") {
			foundErr = true
		}
	}
	assert.True(t, foundErr, "expected a tool-result message carrying the tool error")
}

// TestClient_RunAgent_MaxSteps verifies that when the model keeps requesting
// tool calls, the loop runs exactly MaxSteps iterations and returns normally
// (RunAgent does NOT return ErrMaxSteps — it logs and returns the
// accumulated messages). The last message has tool calls and no final answer.
func TestClient_RunAgent_MaxSteps(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, _ *http.Request) {
		// Always request a tool call; never produce a final answer.
		_ = json.NewEncoder(w).Encode(completionResponse("c",
			toolCallChoice("call_x", "echo", `{"text":"again"}`),
			map[string]any{"prompt_tokens": 5, "completion_tokens": 5, "total_tokens": 10}))
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	resp, err := c.RunAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "loop forever"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     3,
	})
	require.NoError(t, err, "RunAgent does not error on max steps; it returns normally")
	assert.Equal(t, 3, resp.Steps)
	// allMessages ends with tool-result messages from the final step; the
	// last assistant message (second-to-last overall) should still be
	// requesting tool calls — no final answer was produced.
	require.GreaterOrEqual(t, len(resp.Messages), 2)
	lastAssistant := resp.Messages[len(resp.Messages)-2]
	assert.Equal(t, RoleAssistant, lastAssistant.Role)
	assert.NotEmpty(t, lastAssistant.ToolCalls, "last assistant message should still request tool calls")
	// No final text answer: the very last message is a tool result, not an
	// assistant text message.
	assert.Equal(t, RoleTool, resp.Messages[len(resp.Messages)-1].Role)
}

// TestClient_RunAgent_MaxTotalTokens verifies that exceeding the cumulative
// token budget aborts the loop with an error wrapping ErrMaxSteps.
func TestClient_RunAgent_MaxTotalTokens(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, _ *http.Request) {
		// First (and only) step returns a tool call with usage that already
		// exceeds the small budget.
		_ = json.NewEncoder(w).Encode(completionResponse("c1",
			toolCallChoice("call_1", "echo", `{"text":"hi"}`),
			map[string]any{"prompt_tokens": 50, "completion_tokens": 60, "total_tokens": 110}))
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	_, err = c.RunAgent(context.Background(), AgentRequest{
		ModelProfile:   ModelChat,
		Messages:       []Message{{Role: RoleUser, Content: "blow the budget"}},
		Tools:          []Tool{EchoTool},
		MaxSteps:       5,
		MaxTotalTokens: 100, // usage total_tokens=110 exceeds this
	})
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMaxSteps)
	assert.Contains(t, err.Error(), "max total tokens exceeded")
}

// TestClient_RunAgent_MultipleToolCallsInOneStep verifies that when the
// model requests several tool calls in a single assistant message, all of
// them are executed and their results are appended before the next step.
func TestClient_RunAgent_MultipleToolCallsInOneStep(t *testing.T) {
	callCount := 0
	server := mockOpenRouterServer(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		if callCount == 1 {
			_ = json.NewEncoder(w).Encode(completionResponse("c1",
				multiToolCallChoice([]map[string]string{
					{"id": "call_a", "name": "echo", "arguments": `{"text":"first"}`},
					{"id": "call_b", "name": "echo", "arguments": `{"text":"second"}`},
				}),
				map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}))
			return
		}
		_ = json.NewEncoder(w).Encode(completionResponse("c2",
			textChoice("got both echoes"),
			map[string]any{"prompt_tokens": 20, "completion_tokens": 10, "total_tokens": 30}))
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	resp, err := c.RunAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "echo twice"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     5,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Steps)

	// Two tool-result messages should be present, carrying each echo output.
	toolResults := 0
	for _, m := range resp.Messages {
		if m.Role == RoleTool {
			toolResults++
		}
	}
	assert.Equal(t, 2, toolResults, "expected two tool-result messages from one step")
	assert.Contains(t, resp.Messages[len(resp.Messages)-1].Content, "got both echoes")
}

// TestClient_RunAgent_MultiStepChain verifies a 3-step loop: tool call →
// tool call → final answer.
func TestClient_RunAgent_MultiStepChain(t *testing.T) {
	callCount := 0
	server := mockOpenRouterServer(func(w http.ResponseWriter, _ *http.Request) {
		callCount++
		switch callCount {
		case 1:
			_ = json.NewEncoder(w).Encode(completionResponse("c1",
				toolCallChoice("call_1", "echo", `{"text":"step1"}`),
				map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}))
		case 2:
			_ = json.NewEncoder(w).Encode(completionResponse("c2",
				toolCallChoice("call_2", "echo", `{"text":"step2"}`),
				map[string]any{"prompt_tokens": 20, "completion_tokens": 5, "total_tokens": 25}))
		default:
			_ = json.NewEncoder(w).Encode(completionResponse("c3",
				textChoice("final after two tools"),
				map[string]any{"prompt_tokens": 30, "completion_tokens": 10, "total_tokens": 40}))
		}
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	resp, err := c.RunAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "two-step chain"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     5,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, resp.Steps)
	assert.Equal(t, 3, callCount, "three LLM calls expected")
	assert.Contains(t, resp.Messages[len(resp.Messages)-1].Content, "final after two tools")
	// Cumulative usage is summed across all steps.
	assert.Equal(t, 15+25+40, resp.Usage.TotalTokens)
}
