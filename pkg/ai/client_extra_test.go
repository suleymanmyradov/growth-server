package ai

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClient_Generate_WithFallback(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		// Parse the request body to check which model is being called.
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		model, _ := body["model"].(string)

		if model == "deepseek/deepseek-chat-v3" {
			// Primary model returns 500.
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"error":{"message":"overloaded","type":"server_error","code":500}}`))
			return
		}

		// Fallback model succeeds.
		resp := map[string]any{
			"id":      "chatcmpl-fb",
			"object":  "chat.completion",
			"model":   "anthropic/claude-3.5-haiku",
			"choices": []any{map[string]any{"index": 0, "message": map[string]any{"role": "assistant", "content": "fallback response"}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 5, "completion_tokens": 3, "total_tokens": 8},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	cfg.MaxRetries = 0 // Don't retry the primary; go straight to fallback.
	cfg.FallbackPolicy = FallbackPolicy{Enabled: true, MaxFailures: 1}

	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	resp, err := c.Generate(context.Background(), GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
	})
	require.NoError(t, err)
	assert.Contains(t, resp.Message.Content, "fallback")
}

func TestClient_Generate_QuotaExceeded(t *testing.T) {
	cfg := testConfig("test-key")
	cfg.Quota.UserDailyTokenCap = 100

	store := &mockQuotaStore{userOK: false}
	c, err := New(cfg, WithQuotaStore(store))
	require.NoError(t, err)

	_, err = c.Generate(context.Background(), GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
		Metadata:     Metadata{UserID: "user1"},
	})
	assert.ErrorIs(t, err, ErrQuotaExceeded)
}

func TestClient_Generate_InvalidProfile(t *testing.T) {
	cfg := testConfig("test-key")
	c, err := New(cfg)
	require.NoError(t, err)

	_, err = c.Generate(context.Background(), GenerateRequest{
		ModelProfile: ModelProfile("nonexistent"),
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
	})
	assert.ErrorIs(t, err, ErrInvalidProfile)
}

func TestClient_Generate_WithContextCancel(t *testing.T) {
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second) // block forever
	})
	defer server.Close()

	cfg := testConfig("test-key")
	cfg.BaseURL = server.URL
	cfg.MaxRetries = 0

	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err = c.Generate(ctx, GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "Hello"}},
	})
	assert.Error(t, err)
}

func TestOpenRouterTransport_Headers(t *testing.T) {
	var seenHeaders http.Header
	server := mockOpenRouterServer(func(w http.ResponseWriter, r *http.Request) {
		seenHeaders = r.Header
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	})
	defer server.Close()

	cfg := testConfig("test-api-key")
	cfg.BaseURL = server.URL

	c, err := New(cfg, WithHTTPClient(server.Client()))
	require.NoError(t, err)

	_, _ = c.Generate(context.Background(), GenerateRequest{
		ModelProfile: ModelCheap,
		Messages:     []Message{{Role: RoleUser, Content: "test"}},
	})

	assert.Equal(t, "Bearer test-api-key", seenHeaders.Get("Authorization"))
	assert.Equal(t, "https://growth.app", seenHeaders.Get("HTTP-Referer"))
	assert.Equal(t, "growth", seenHeaders.Get("X-Title"))
}

// mockQuotaStore is a test double for QuotaStore.
type mockQuotaStore struct {
	userOK    bool
	globalOK  bool
	userErr   error
	globalErr error
}

func (m *mockQuotaStore) CheckUserQuota(_ context.Context, _ string, _ int64) (bool, error) {
	return m.userOK, m.userErr
}
func (m *mockQuotaStore) IncrUserTokens(_ context.Context, _ string, _ int64) error { return nil }
func (m *mockQuotaStore) CheckGlobalQuota(_ context.Context, _ int64) (bool, error) {
	return m.globalOK, m.globalErr
}
func (m *mockQuotaStore) IncrGlobalCost(_ context.Context, _ int64) error { return nil }

func TestCheckQuota_UserQuotaExceeded(t *testing.T) {
	cfg := testConfig("test-key")
	cfg.Quota.UserDailyTokenCap = 100
	store := &mockQuotaStore{userOK: false}
	c, err := New(cfg, WithQuotaStore(store))
	require.NoError(t, err)

	cl := c.(*client)
	err = cl.checkQuota(context.Background(), Metadata{UserID: "user1"})
	assert.ErrorIs(t, err, ErrQuotaExceeded)
}

func TestCheckQuota_GlobalQuotaExceeded(t *testing.T) {
	cfg := testConfig("test-key")
	cfg.Quota.GlobalDailyCostCapUSD = 5.0
	store := &mockQuotaStore{userOK: true, globalOK: false}
	c, err := New(cfg, WithQuotaStore(store))
	require.NoError(t, err)

	cl := c.(*client)
	err = cl.checkQuota(context.Background(), Metadata{UserID: "user1"})
	assert.ErrorIs(t, err, ErrQuotaExceeded)
}

func TestCheckQuota_NoStore(t *testing.T) {
	cfg := testConfig("test-key")
	c, err := New(cfg)
	require.NoError(t, err)

	cl := c.(*client)
	err = cl.checkQuota(context.Background(), Metadata{UserID: "user1"})
	assert.NoError(t, err)
}

func TestCheckQuota_StoreError_FailOpen(t *testing.T) {
	cfg := testConfig("test-key")
	cfg.Quota.UserDailyTokenCap = 100
	store := &mockQuotaStore{userErr: context.DeadlineExceeded}
	c, err := New(cfg, WithQuotaStore(store))
	require.NoError(t, err)

	cl := c.(*client)
	err = cl.checkQuota(context.Background(), Metadata{UserID: "user1"})
	assert.NoError(t, err) // fail open
}

func TestToFromEinoMessage(t *testing.T) {
	original := Message{
		Role:    RoleAssistant,
		Content: "Hello!",
		ToolCalls: []ToolCall{
			{ID: "call_1", Type: "function", Fn: FunctionCall{Name: "echo", Arguments: `{"text":"hi"}`}},
		},
	}

	einoMsg := toEinoMessage(original)
	assert.Equal(t, "assistant", string(einoMsg.Role))
	assert.Equal(t, "Hello!", einoMsg.Content)
	assert.Len(t, einoMsg.ToolCalls, 1)

	roundTrip := fromEinoMessage(einoMsg)
	assert.Equal(t, original.Role, roundTrip.Role)
	assert.Equal(t, original.Content, roundTrip.Content)
	assert.Equal(t, original.ToolCalls[0].ID, roundTrip.ToolCalls[0].ID)
}

func TestToFromEinoMessage_ToolResult(t *testing.T) {
	original := Message{
		Role:       RoleTool,
		Content:    `{"echo":"hi"}`,
		ToolCallID: "call_1",
	}

	einoMsg := toEinoMessage(original)
	assert.Equal(t, "tool", string(einoMsg.Role))
	assert.Equal(t, "call_1", einoMsg.ToolCallID)

	roundTrip := fromEinoMessage(einoMsg)
	assert.Equal(t, original.ToolCallID, roundTrip.ToolCallID)
}

func TestEinoModelOptions(t *testing.T) {
	temp := float32(0.7)
	maxTok := 500
	opts := einoModelOptions(GenerateRequest{
		Temperature: &temp,
		MaxTokens:   &maxTok,
	})
	assert.Len(t, opts, 2)

	// No options.
	opts = einoModelOptions(GenerateRequest{})
	assert.Len(t, opts, 0)
}

func TestBuildToolInfos(t *testing.T) {
	infos := buildToolInfos([]Tool{EchoTool})
	assert.Len(t, infos, 1)
	assert.Equal(t, "echo", infos[0].Name)
}

func TestExtractToolCallArgs(t *testing.T) {
	m := extractToolCallArgs(`{"text":"hello"}`)
	assert.Equal(t, "hello", m["text"])

	m = extractToolCallArgs(`invalid json`)
	assert.Nil(t, m)
}
