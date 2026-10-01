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

func TestClient_Stream(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer test-key", r.Header.Get("Authorization"))

		w.Header().Set("Content-Type", "text/event-stream")
		// Send SSE chunks.
		chunks := []map[string]any{
			{"id": "chatcmpl-1", "object": "chat.completion.chunk", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "Hel"}, "finish_reason": nil}}},
			{"id": "chatcmpl-2", "object": "chat.completion.chunk", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "lo!"}, "finish_reason": "stop"}}},
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

	sr, err := c.Stream(context.Background(), GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
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

// quotaTestConfig returns a config with token/cost caps and a recording store
// so tests can assert usage was actually recorded.
func quotaTestConfig(base string) (Config, *mockQuotaStore) {
	cfg := testConfig("test-key")
	cfg.BaseURL = base
	cfg.Quota.UserDailyTokenCap = 100000
	cfg.Quota.GlobalDailyCostCapUSD = 5.0
	store := &mockQuotaStore{userOK: true, globalOK: true}
	return cfg, store
}

func TestClient_Stream_RecordsUsageOnNormalEOF(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []map[string]any{
			{"id": "c1", "object": "chat.completion.chunk", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "Hel"}, "finish_reason": nil}}},
			{"id": "c2", "object": "chat.completion.chunk", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "lo"}, "finish_reason": "stop"}}, "usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}},
		}
		for _, c := range chunks {
			data, _ := json.Marshal(c)
			_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
		}
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	})
	defer server.Close()

	cfg, store := quotaTestConfig(server.URL)
	c, err := New(cfg, WithHTTPClient(server.Client()), WithQuotaStore(store))
	require.NoError(t, err)

	sr, err := c.Stream(context.Background(), GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
		Metadata:     Metadata{UserID: "user-1"},
	})
	require.NoError(t, err)
	defer sr.Close()

	for {
		if _, err := sr.Recv(); err == io.EOF {
			break
		} else {
			require.NoError(t, err)
		}
	}
	assert.Equal(t, int64(15), store.userTokens.Load())
	// Global cost counter was written too (0$ for free test models, but the
	// increment path ran — the assertion above proves recordUsage fired).
}

// TestClient_Stream_RecordsPartialUsageOnEarlyClose covers the A3 gap: a
// client that aborts a stream mid-flight must still pay for the tokens the
// provider already generated — estimated when the provider hasn't reported.
func TestClient_Stream_RecordsPartialUsageOnEarlyClose(t *testing.T) {
	release := make(chan struct{})
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunk := map[string]any{"id": "c1", "object": "chat.completion.chunk", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "partial answer text"}, "finish_reason": nil}}}
		data, _ := json.Marshal(chunk)
		_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		// Hold the stream open until the client disconnects.
		select {
		case <-r.Context().Done():
		case <-release:
		}
	})
	defer server.Close()
	defer close(release)

	cfg, store := quotaTestConfig(server.URL)
	c, err := New(cfg, WithHTTPClient(server.Client()), WithQuotaStore(store))
	require.NoError(t, err)

	sr, err := c.Stream(context.Background(), GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
		Metadata:     Metadata{UserID: "user-1"},
	})
	require.NoError(t, err)

	chunk, err := sr.Recv()
	require.NoError(t, err)
	assert.Equal(t, "partial answer text", chunk.Delta)

	// Early close before EOF — partial usage must still be recorded.
	sr.Close()
	assert.Greater(t, store.userTokens.Load(), int64(0), "early-closed stream must record estimated usage")
}

// TestClient_Stream_RecordsPartialUsageOnContextCancel covers the client-
// disconnect path: ctx cancellation mid-stream records what was generated.
func TestClient_Stream_RecordsPartialUsageOnContextCancel(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunk := map[string]any{"id": "c1", "object": "chat.completion.chunk", "choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": "half an answer"}, "finish_reason": nil}}}
		data, _ := json.Marshal(chunk)
		_, _ = w.Write([]byte("data: " + string(data) + "\n\n"))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		<-r.Context().Done() // hold until client cancels
	})
	defer server.Close()

	cfg, store := quotaTestConfig(server.URL)
	c, err := New(cfg, WithHTTPClient(server.Client()), WithQuotaStore(store))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	sr, err := c.Stream(ctx, GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
		Metadata:     Metadata{UserID: "user-1"},
	})
	require.NoError(t, err)

	_, err = sr.Recv()
	require.NoError(t, err)

	cancel() // simulate client disconnect
	_, _ = sr.Recv()
	sr.Close()
	assert.Greater(t, store.userTokens.Load(), int64(0), "canceled stream must record partial usage")
}
