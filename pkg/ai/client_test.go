package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig(apiKey string) Config {
	return Config{
		APIKey:         apiKey,
		BaseURL:        "https://openrouter.ai/api/v1", // overridden by httptest
		DefaultTimeout: 10 * time.Second,
		MaxRetries:     1,
		RetryBackoff:   100 * time.Millisecond,
		HTTPReferer:    "https://growth.app",
		XTitle:         "growth",
		Models: map[string]string{
			string(ModelCheap):      "nvidia/nemotron-nano-9b-v2:free",
			string(ModelCheapLong):  "nvidia/nemotron-3-ultra-550b-a55b:free",
			string(ModelClassifier): "nvidia/nemotron-nano-9b-v2:free",
			string(ModelChat):       "qwen/qwen3-next-80b-a3b-instruct:free",
			string(ModelFallback):   "meta-llama/llama-3.3-70b-instruct:free",
		},
		CostRates: map[string]CostRate{
			"nvidia/nemotron-nano-9b-v2:free":        {PromptPer1K: 0, CompletionPer1K: 0},
			"nvidia/nemotron-3-ultra-550b-a55b:free": {PromptPer1K: 0, CompletionPer1K: 0},
			"qwen/qwen3-next-80b-a3b-instruct:free":  {PromptPer1K: 0, CompletionPer1K: 0},
			"meta-llama/llama-3.3-70b-instruct:free": {PromptPer1K: 0, CompletionPer1K: 0},
		},
	}
}

// mockOpenRouterServer creates an httptest.Server that simulates OpenRouter responses.
func mockOpenRouterServer(handler http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(handler)
}

func TestConfig_Validate(t *testing.T) {
	allProfiles := map[string]string{
		string(ModelCheap):      "nvidia/nemotron-nano-9b-v2:free",
		string(ModelCheapLong):  "nvidia/nemotron-3-ultra-550b-a55b:free",
		string(ModelClassifier): "nvidia/nemotron-nano-9b-v2:free",
		string(ModelChat):       "qwen/qwen3-next-80b-a3b-instruct:free",
		string(ModelFallback):   "meta-llama/llama-3.3-70b-instruct:free",
	}

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name:    "missing API key",
			cfg:     Config{},
			wantErr: true,
		},
		{
			name:    "missing models",
			cfg:     Config{APIKey: "test-key"},
			wantErr: true,
		},
		{
			name: "missing one profile",
			cfg: Config{
				APIKey: "test-key",
				Models: func() map[string]string {
					m := make(map[string]string, len(allProfiles))
					for k, v := range allProfiles {
						m[k] = v
					}
					delete(m, string(ModelChat))
					return m
				}(),
			},
			wantErr: true,
		},
		{
			name: "valid with all profiles",
			cfg: Config{
				APIKey: "test-key",
				Models: allProfiles,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, tt.cfg.BaseURL)
			assert.NotZero(t, tt.cfg.DefaultTimeout)
			assert.NotZero(t, tt.cfg.MaxRetries)
		})
	}
}

func TestConfig_ModelID(t *testing.T) {
	cfg := Config{
		APIKey: "test",
		Models: map[string]string{
			string(ModelCheap):      "nvidia/nemotron-nano-9b-v2:free",
			string(ModelCheapLong):  "nvidia/nemotron-3-ultra-550b-a55b:free",
			string(ModelClassifier): "nvidia/nemotron-nano-9b-v2:free",
			string(ModelChat):       "qwen/qwen3-next-80b-a3b-instruct:free",
			string(ModelFallback):   "meta-llama/llama-3.3-70b-instruct:free",
		},
	}
	require.NoError(t, cfg.Validate())

	id, err := cfg.ModelID(ModelCheap)
	require.NoError(t, err)
	assert.Equal(t, "nvidia/nemotron-nano-9b-v2:free", id)

	_, err = cfg.ModelID(ModelProfile("nonexistent"))
	assert.Error(t, err)
}

func TestConfig_ComputeCost(t *testing.T) {
	cfg := Config{APIKey: "test", CostRates: map[string]CostRate{}}

	// Free models have zero cost.
	cost := cfg.ComputeCost("nvidia/nemotron-nano-9b-v2:free", 1000, 1000)
	assert.Equal(t, 0.0, cost)

	// Unknown models also return zero cost.
	cost = cfg.ComputeCost("unknown-model", 1000, 1000)
	assert.Equal(t, 0.0, cost)

	// Custom cost rate is respected.
	cfg.CostRates["custom/paid-model"] = CostRate{PromptPer1K: 0.001, CompletionPer1K: 0.002}
	cost = cfg.ComputeCost("custom/paid-model", 1000, 1000)
	assert.Equal(t, 0.003, cost)
}

func TestNew_MissingAPIKey(t *testing.T) {
	_, err := New(Config{})
	assert.Error(t, err)
}

func TestNew_WithCustomHTTPClient(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`))
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL

	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)
	require.NotNil(t, c)
}

func TestClient_Generate(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))
		assert.Equal(t, "https://growth.app", r.Header.Get("HTTP-Referer"))
		assert.Equal(t, "growth", r.Header.Get("X-Title"))

		resp := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"model":   "nvidia/nemotron-nano-9b-v2:free",
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "Great work!"}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL

	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	resp, err := c.Generate(context.Background(), GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
	})
	require.NoError(t, err)
	assert.Equal(t, "Great work!", resp.Message.Content)
	assert.Equal(t, 10, resp.Usage.PromptTokens)
	assert.Equal(t, 5, resp.Usage.CompletionTokens)
	assert.Equal(t, 0.0, resp.CostUSD) // free model
}

func TestClient_Generate_APIError(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"internal error","type":"server_error","code":500}}`))
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	cfg.MaxRetries = 1

	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	_, err = c.Generate(context.Background(), GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
	})
	assert.Error(t, err)
}

func TestClient_GenerateStructured(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"model":   "nvidia/nemotron-nano-9b-v2:free",
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": `{"summary":"good week","blockers":"none"}`}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL

	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	var result struct {
		Summary  string `json:"summary"`
		Blockers string `json:"blockers"`
	}
	err = c.GenerateStructured(context.Background(), GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "Review my week"}},
	}, &result)
	require.NoError(t, err)
	assert.Equal(t, "good week", result.Summary)
	assert.Equal(t, "none", result.Blockers)
}

func TestClient_GenerateStructured_MarkdownFence(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		content := "```json\n{\"key\":\"value\"}\n```"
		resp := map[string]any{
			"id":      "chatcmpl-test",
			"object":  "chat.completion",
			"model":   "nvidia/nemotron-nano-9b-v2:free",
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": content}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 5, "total_tokens": 10},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL

	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	var result struct {
		Key string `json:"key"`
	}
	err = c.GenerateStructured(context.Background(), GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "test"}},
	}, &result)
	require.NoError(t, err)
	assert.Equal(t, "value", result.Key)
}

func TestErrors(t *testing.T) {
	t.Run("QuotaError", func(t *testing.T) {
		e := &QuotaError{Limit: "user_daily", Used: 1000, Cap: 500}
		assert.ErrorIs(t, e, ErrQuotaExceeded)
		assert.Contains(t, e.Error(), "user_daily")
	})

	t.Run("SafetyError", func(t *testing.T) {
		e := &SafetyError{Category: "self_harm", Confidence: 0.95, Reason: "test"}
		assert.ErrorIs(t, e, ErrSafetyBlock)
		assert.Contains(t, e.Error(), "self_harm")
	})
}

func TestIsRetryable(t *testing.T) {
	assert.True(t, isRetryable(&apiError{StatusCode: 429}))
	assert.True(t, isRetryable(&apiError{StatusCode: 500}))
	assert.True(t, isRetryable(&apiError{StatusCode: 502}))
	assert.False(t, isRetryable(&apiError{StatusCode: 400}))
	assert.False(t, isRetryable(&apiError{StatusCode: 401}))
	assert.False(t, isRetryable(context.Canceled))
	assert.False(t, isRetryable(context.DeadlineExceeded))
	assert.False(t, isRetryable(nil))
}

func TestUnwrapAPIError(t *testing.T) {
	inner := &apiError{StatusCode: 500, Message: "oops"}
	wrapped := fmt.Errorf("wrap: %w", inner)

	ae, ok := unwrapAPIError(wrapped)
	assert.True(t, ok)
	assert.Equal(t, 500, ae.StatusCode)
}

func TestExtractUsage(t *testing.T) {
	t.Run("nil response meta", func(t *testing.T) {
		u := extractUsage(&einoMessage{})
		assert.Equal(t, Usage{}, u)
	})
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello", truncate("hello", 10))
	assert.Equal(t, "hel...", truncate("hello world", 3))
}
