package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient_Generate_CrossProviderFallback verifies that when the primary
// provider fails and FallbackProviders is configured, the fallback request
// routes to the fallback provider's base URL with the fallback provider's
// API key and model — not the primary provider's.
func TestClient_Generate_CrossProviderFallback(t *testing.T) {
	var primaryCalled, fallbackCalled bool
	var fallbackAuthHeader, fallbackModel string

	// Primary server: always returns 500.
	primaryServer := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		primaryCalled = true
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"nvidia overloaded","type":"server_error","code":500}}`))
	})
	defer primaryServer.Close()

	// Fallback server: returns a successful response.
	fallbackServer := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		fallbackCalled = true
		fallbackAuthHeader = r.Header.Get("Authorization")

		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		fallbackModel, _ = body["model"].(string)

		resp := map[string]any{
			"id":      "chatcmpl-fb",
			"object":  "chat.completion",
			"model":   fallbackModel,
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "google fallback response"}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
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
			string(ModelCheap):      "gemini-flash-lite-latest",
			string(ModelChat):       "gemini-flash-latest",
			string(ModelFallback):   "gemini-flash-lite-latest",
			string(ModelCheapLong):  "gemini-flash-latest",
			string(ModelClassifier): "gemini-flash-lite-latest",
		},
	}}

	c, err := New(cfg, WithHTTPClient(primaryServer.Client()))
	require.NoError(t, err)

	resp, err := c.Generate(context.Background(), GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Message.Content, "google fallback")

	// Verify the primary was attempted and the fallback was used.
	assert.True(t, primaryCalled, "primary server should have been called")
	assert.True(t, fallbackCalled, "fallback server should have been called")

	// Verify the fallback used the fallback provider's API key, not the primary's.
	assert.Equal(t, "Bearer google-key", fallbackAuthHeader,
		"fallback request should use the fallback provider's API key")

	// Verify the fallback used the fallback provider's model for the cheap profile.
	assert.Equal(t, "gemini-flash-lite-latest", fallbackModel,
		"fallback request should use the fallback provider's model for the requested profile")
}

// TestClient_Generate_FallbackChain verifies that when multiple fallback
// providers are configured, they are tried in order: the first fallback that
// succeeds wins. This simulates the real scenario where the first Google
// model (flash-lite) is rate-limited (429) and the second (flash) succeeds.
func TestClient_Generate_FallbackChain(t *testing.T) {
	var (
		primaryCalls     atomic.Int64
		fallback1Calls   atomic.Int64
		fallback2Calls   atomic.Int64
		fallback2Model   string
		fallback2AuthKey string
	)

	// Primary server: always returns 500.
	primaryServer := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		primaryCalls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":{"message":"nvidia overloaded","type":"server_error","code":500}}`))
	})
	defer primaryServer.Close()

	// Fallback 1 server: always returns 429 (rate-limited).
	fallback1Server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		fallback1Calls.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"Quota exceeded. Please retry in 35s.","type":"rate_limit","code":429}}`))
	})
	defer fallback1Server.Close()

	// Fallback 2 server: succeeds.
	fallback2Server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		fallback2Calls.Add(1)
		fallback2AuthKey = r.Header.Get("Authorization")

		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		fallback2Model, _ = body["model"].(string)

		resp := map[string]any{
			"id":      "chatcmpl-fb2",
			"object":  "chat.completion",
			"model":   fallback2Model,
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "second fallback response"}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer fallback2Server.Close()

	cfg := testConfig("primary-key")
	cfg.BaseURL = primaryServer.URL
	cfg.MaxRetries = 0 // Don't retry; go straight through the chain.
	cfg.FallbackPolicy = FallbackPolicy{Enabled: true, MaxFailures: 1}
	cfg.FallbackProviders = []ProviderConfig{
		{
			APIKey:  "google-key-1",
			BaseURL: fallback1Server.URL,
			Models: map[string]string{
				string(ModelCheap): "gemini-flash-lite-latest",
			},
		},
		{
			APIKey:  "google-key-2",
			BaseURL: fallback2Server.URL,
			Models: map[string]string{
				string(ModelCheap): "gemini-flash-latest",
			},
		},
	}

	c, err := New(cfg, WithHTTPClient(primaryServer.Client()))
	require.NoError(t, err)

	resp, err := c.Generate(context.Background(), GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Message.Content, "second fallback")

	// All three servers should have been called (primary → fb1 → fb2).
	assert.Equal(t, int64(1), primaryCalls.Load(), "primary should be called once")
	assert.Equal(t, int64(1), fallback1Calls.Load(), "fallback 1 should be called once")
	assert.Equal(t, int64(1), fallback2Calls.Load(), "fallback 2 should be called once")

	// Fallback 2 should have used the second provider's API key and model.
	assert.Equal(t, "Bearer google-key-2", fallback2AuthKey)
	assert.Equal(t, "gemini-flash-latest", fallback2Model)
}

// TestClient_Generate_CrossProviderFallback_NotConfigured verifies that
// without FallbackProviders, the same-provider ModelFallback profile is used.
func TestClient_Generate_CrossProviderFallback_NotConfigured(t *testing.T) {
	// Single server handles both primary and same-provider fallback.
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		model, _ := body["model"].(string)

		if model == "nvidia/nemotron-nano-9b-v2:free" {
			// Primary model returns 500.
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"overloaded","type":"server_error","code":500}}`))
			return
		}

		// Same-provider fallback succeeds.
		resp := map[string]any{
			"id":      "chatcmpl-fb",
			"object":  "chat.completion",
			"model":   model,
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "same-provider fallback"}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	cfg.MaxRetries = 0
	cfg.FallbackPolicy = FallbackPolicy{Enabled: true, MaxFailures: 1}
	// No FallbackProviders configured — should use same-provider ModelFallback.

	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	resp, err := c.Generate(context.Background(), GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Message.Content, "same-provider fallback")
}

// TestConfig_FallbackProviders_Validation tests validation of the
// FallbackProviders config.
func TestConfig_FallbackProviders_Validation(t *testing.T) {
	baseModels := map[string]string{
		string(ModelCheap):      "nvidia/nemotron-nano-9b-v2:free",
		string(ModelCheapLong):  "nvidia/nemotron-3-ultra-550b-a55b:free",
		string(ModelClassifier): "nvidia/nemotron-nano-9b-v2:free",
		string(ModelChat):       "qwen/qwen3-next-80b-a3b-instruct:free",
		string(ModelFallback):   "meta-llama/llama-3.3-70b-instruct:free",
	}

	t.Run("missing api key", func(t *testing.T) {
		cfg := Config{
			APIKey: "primary-key",
			Models: baseModels,
			FallbackProviders: []ProviderConfig{{
				BaseURL: "https://example.com/v1",
				Models:  map[string]string{"cheap": "model-x"},
			}},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "FallbackProviders[0].APIKey")
	})

	t.Run("missing models", func(t *testing.T) {
		cfg := Config{
			APIKey: "primary-key",
			Models: baseModels,
			FallbackProviders: []ProviderConfig{{
				APIKey:  "fb-key",
				BaseURL: "https://example.com/v1",
			}},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "FallbackProviders[0].Models")
	})

	t.Run("valid fallback provider", func(t *testing.T) {
		cfg := Config{
			APIKey: "primary-key",
			Models: baseModels,
			FallbackProviders: []ProviderConfig{{
				APIKey:  "fb-key",
				BaseURL: "https://example.com/v1",
				Models:  map[string]string{"cheap": "model-x"},
			}},
		}
		err := cfg.Validate()
		require.NoError(t, err)
	})

	t.Run("default base url", func(t *testing.T) {
		cfg := Config{
			APIKey: "primary-key",
			Models: baseModels,
			FallbackProviders: []ProviderConfig{{
				APIKey: "fb-key",
				Models: map[string]string{"cheap": "model-x"},
			}},
		}
		err := cfg.Validate()
		require.NoError(t, err)
		assert.Equal(t, "https://openrouter.ai/api/v1", cfg.FallbackProviders[0].BaseURL)
	})

	t.Run("second provider missing key", func(t *testing.T) {
		cfg := Config{
			APIKey: "primary-key",
			Models: baseModels,
			FallbackProviders: []ProviderConfig{
				{APIKey: "fb-key-1", BaseURL: "https://a.com", Models: map[string]string{"cheap": "m1"}},
				{APIKey: "", BaseURL: "https://b.com", Models: map[string]string{"cheap": "m2"}},
			},
		}
		err := cfg.Validate()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "FallbackProviders[1].APIKey")
	})
}

// TestParseRetryDelay tests extraction of retry delays from provider error
// messages, including Google AI Studio's 429 format.
func TestParseRetryDelay(t *testing.T) {
	tests := []struct {
		name    string
		err     error
		want    time.Duration
		wantAny bool // if true, just check > 0
	}{
		{
			name: "google 429 message",
			err:  &apiError{StatusCode: 429, Message: "You exceeded your current quota. Please retry in 35.683551572s."},
			want: 35 * time.Second,
		},
		{
			name: "google 429 with retryDelay field",
			err:  &apiError{StatusCode: 429, Message: `quota exceeded, retry_delay: "42s"`},
			want: 42 * time.Second,
		},
		{
			name: "nvidia 429 no delay",
			err:  &apiError{StatusCode: 429, Message: "Too Many Requests"},
			want: 0,
		},
		{
			name: "no retry info",
			err:  &apiError{StatusCode: 500, Message: "internal error"},
			want: 0,
		},
		{
			name: "nil error",
			err:  nil,
			want: 0,
		},
		{
			name: "capped at 60s",
			err:  &apiError{StatusCode: 429, Message: "Please retry in 120s."},
			want: 60 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseRetryDelay(tt.err)
			if tt.wantAny {
				assert.Greater(t, got, time.Duration(0))
				return
			}
			// Allow small rounding differences for float parsing.
			if tt.want > 0 {
				assert.InDelta(t, tt.want.Seconds(), got.Seconds(), 1.0)
			} else {
				assert.Equal(t, time.Duration(0), got)
			}
		})
	}
}

// TestParseRetryDelayFromString tests the string parser directly.
func TestParseRetryDelayFromString(t *testing.T) {
	assert.InDelta(t, 35, parseRetryDelayFromString("Please retry in 35.683551572s.").Seconds(), 1.0)
	assert.Equal(t, 42*time.Second, parseRetryDelayFromString(`retry_delay: "42s"`))
	assert.Equal(t, time.Duration(0), parseRetryDelayFromString("no retry info here"))
	assert.Equal(t, time.Duration(0), parseRetryDelayFromString(""))
	assert.Equal(t, 5*time.Second, parseRetryDelayFromString("retry in 5s"))
	assert.Equal(t, 60*time.Second, parseRetryDelayFromString("retry in 999s")) // capped
}
