package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_RunAgent(t *testing.T) {
	callCount := 0
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

		if callCount == 1 {
			// First call: model requests a tool call.
			resp := map[string]any{
				"id":     "chatcmpl-1",
				"object": "chat.completion",
				"model":  "openai/gpt-4o-mini",
				"choices": []any{map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "",
						"tool_calls": []any{map[string]any{
							"id":   "call_1",
							"type": "function",
							"function": map[string]any{
								"name":      "echo",
								"arguments": `{"text":"hello"}`,
							},
						}},
					},
					"finish_reason": "tool_calls",
				}},
				"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
			}
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		// Second call: model responds with final answer.
		resp := map[string]any{
			"id":     "chatcmpl-2",
			"object": "chat.completion",
			"model":  "openai/gpt-4o-mini",
			"choices": []any{map[string]any{
				"index": 0,
				"message": map[string]any{
					"role":    "assistant",
					"content": "The echo tool returned: hello",
				},
				"finish_reason": "stop",
			}},
			"usage": map[string]any{"prompt_tokens": 20, "completion_tokens": 10, "total_tokens": 30},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL

	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	resp, err := c.RunAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "Use the echo tool"}},
		Tools:        []Tool{EchoTool},
		MaxSteps:     5,
	})
	require.NoError(t, err)
	assert.Equal(t, 2, resp.Steps)
	assert.GreaterOrEqual(t, len(resp.Messages), 3) // tool_call + tool_result + final
	assert.Contains(t, resp.Messages[len(resp.Messages)-1].Content, "hello")
}

func TestClient_RunAgent_NoTools(t *testing.T) {
	cfg := testConfig("test-key")
	c, err := New(cfg)
	require.NoError(t, err)

	_, err = c.RunAgent(context.Background(), AgentRequest{
		ModelProfile: ModelChat,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
		Tools:        []Tool{},
		MaxSteps:     5,
	})
	assert.ErrorIs(t, err, ErrNoTools)
}
