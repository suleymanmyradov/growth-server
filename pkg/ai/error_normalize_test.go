package ai

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestClient_Generate_ArrayWrappedError verifies that when a provider
// (Google's Gemini OpenAI-compat endpoint) returns an error as a JSON
// array like [{"error":{"code":400,"message":"Function call..."}}], the
// error is properly parsed and the real error message ("Function call...")
// is surfaced — not masked behind a "cannot unmarshal array into Go value
// of type openai.ErrorResponse" JSON parse error.
//
// This is a known Gemini behavior: some error responses are array-wrapped
// instead of object-wrapped, which breaks the go-openai client's
// ErrorResponse unmarshaling.
func TestClient_Generate_ArrayWrappedError(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		// Google returns array-wrapped errors: [{ "error": { ... } }]
		_, _ = w.Write([]byte(`[{"error":{"code":400,"message":"Function call is missing a thought_signature in functionCall parts.","status":"INVALID_ARGUMENT"}}]`))
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	cfg.MaxRetries = 0 // Don't retry; we want to see the error immediately.

	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	_, err = c.Generate(context.Background(), GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
	})
	require.Error(t, err)

	errStr := err.Error()
	// The real error message must be visible, not buried under a JSON
	// unmarshal error.
	assert.Contains(t, errStr, "Function call is missing a thought_signature",
		"error should contain the provider's real message, not a JSON parse error")
	assert.NotContains(t, errStr, "cannot unmarshal array",
		"error should not contain the JSON unmarshal failure that masks the real error")
}

// TestClient_Generate_ArrayWrappedError503 verifies the same normalization
// works for 503 errors (the "high demand" scenario from the original
// incident).
func TestClient_Generate_ArrayWrappedError503(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`[{"error":{"code":503,"message":"This model is currently experiencing high demand","status":"UNAVAILABLE"}}]`))
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	cfg.MaxRetries = 0

	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	_, err = c.Generate(context.Background(), GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
	})
	require.Error(t, err)

	errStr := err.Error()
	assert.Contains(t, errStr, "high demand",
		"error should contain the provider's real 503 message")
	assert.NotContains(t, errStr, "cannot unmarshal array",
		"error should not contain the JSON unmarshal failure")
}

// TestClient_Generate_ArrayWrappedError_StatusVisibleToRetryLogic verifies
// that after normalization, the HTTP status code is visible to isRetryable
// — a 400 should NOT be retryable (caller bug), while a 503 should be.
// Before the fix, the array-wrapped error was a RequestError (unmarshal
// failure) that isRetryable couldn't parse, so it defaulted to retryable=true
// and retried 400s wastefully.
func TestClient_Generate_ArrayWrappedError_StatusVisibleToRetryLogic(t *testing.T) {
	t.Run("400 not retryable", func(t *testing.T) {
		server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`[{"error":{"code":400,"message":"bad request","status":"INVALID_ARGUMENT"}}]`))
		})
		defer server.Close()

		cfg := testConfig("test-key")
		cfg.BaseURL = server.URL
		cfg.MaxRetries = 0

		c, err := New(cfg, WithHTTPClient(server.Client()))
		require.NoError(t, err)

		_, err = c.Generate(context.Background(), GenerateRequest{
			ModelProfile: ModelCheap,
			Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
		})
		require.Error(t, err)

		// After normalization, the error should be an *APIError (eino-ext)
		// with HTTPStatusCode=400, which isRetryable recognizes as
		// non-retryable. Verify isRetryable directly.
		assert.False(t, isRetryable(err),
			"400 should not be retryable — isRetryable should see the status code after normalization")
	})

	t.Run("503 retryable", func(t *testing.T) {
		server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`[{"error":{"code":503,"message":"overloaded","status":"UNAVAILABLE"}}]`))
		})
		defer server.Close()

		cfg := testConfig("test-key")
		cfg.BaseURL = server.URL
		cfg.MaxRetries = 0

		c, err := New(cfg, WithHTTPClient(server.Client()))
		require.NoError(t, err)

		_, err = c.Generate(context.Background(), GenerateRequest{
			ModelProfile: ModelCheap,
			Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
		})
		require.Error(t, err)

		// 503 should be retryable — isRetryable should see the status code
		// and return true.
		assert.True(t, isRetryable(err),
			"503 should be retryable — isRetryable should see the status code after normalization")
	})
}

// TestNormalizeArrayWrappedErrorBody tests the body normalization helper
// directly.
func TestNormalizeArrayWrappedErrorBody(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		want    string
		changed bool
	}{
		{
			name:    "array with single error object",
			body:    `[{"error":{"code":400,"message":"bad"}}]`,
			want:    `{"error":{"code":400,"message":"bad"}}`,
			changed: true,
		},
		{
			name:    "array with multiple elements (unwrap first)",
			body:    `[{"error":{"code":503,"message":"overloaded"}},{"extra":"ignored"}]`,
			want:    `{"error":{"code":503,"message":"overloaded"}}`,
			changed: true,
		},
		{
			name:    "single object error (no change)",
			body:    `{"error":{"code":400,"message":"bad"}}`,
			want:    `{"error":{"code":400,"message":"bad"}}`,
			changed: false,
		},
		{
			name:    "empty array (no change)",
			body:    `[]`,
			want:    `[]`,
			changed: false,
		},
		{
			name:    "non-JSON (no change)",
			body:    `not json`,
			want:    `not json`,
			changed: false,
		},
		{
			name:    "array without error key (no change)",
			body:    `[{"foo":"bar"}]`,
			want:    `[{"foo":"bar"}]`,
			changed: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, changed := normalizeArrayWrappedErrorBody([]byte(tt.body))
			assert.Equal(t, tt.want, string(got))
			assert.Equal(t, tt.changed, changed)
		})
	}
}
