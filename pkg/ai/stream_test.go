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
