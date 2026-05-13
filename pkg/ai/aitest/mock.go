package aitest

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
)

// ResponseRecorder records a canned response for a given model profile.
type ResponseRecorder struct {
	Profile ai.ModelProfile
	Message ai.Message
	Usage   ai.Usage
	CostUSD float64
}

// MockClient implements ai.Client with recordable responses for testing.
type MockClient struct {
	mu        sync.Mutex
	responses map[ai.ModelProfile][]ResponseRecorder
	calls     []ai.GenerateRequest
	streams   map[ai.ModelProfile][]ai.Chunk
	err       error // if set, all calls return this error
}

// NewMockClient creates a new MockClient.
func NewMockClient() *MockClient {
	return &MockClient{
		responses: make(map[ai.ModelProfile][]ResponseRecorder),
		streams:   make(map[ai.ModelProfile][]ai.Chunk),
	}
}

// SetError makes all subsequent calls return this error.
func (m *MockClient) SetError(err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.err = err
}

// RecordResponse adds a canned response for a model profile.
func (m *MockClient) RecordResponse(profile ai.ModelProfile, msg ai.Message, usage ai.Usage, costUSD float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[profile] = append(m.responses[profile], ResponseRecorder{
		Profile: profile,
		Message: msg,
		Usage:   usage,
		CostUSD: costUSD,
	})
}

// RecordStream adds canned stream chunks for a model profile.
func (m *MockClient) RecordStream(profile ai.ModelProfile, chunks ...ai.Chunk) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.streams[profile] = append(m.streams[profile], chunks...)
}

// Calls returns all recorded Generate calls.
func (m *MockClient) Calls() []ai.GenerateRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]ai.GenerateRequest, len(m.calls))
	copy(out, m.calls)
	return out
}

// Generate returns the next canned response for the requested profile.
func (m *MockClient) Generate(_ context.Context, req ai.GenerateRequest) (ai.GenerateResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return ai.GenerateResponse{}, m.err
	}

	m.calls = append(m.calls, req)

	recorders := m.responses[req.ModelProfile]
	if len(recorders) == 0 {
		return ai.GenerateResponse{}, fmt.Errorf("aitest: no recorded response for profile %q", req.ModelProfile)
	}

	rec := recorders[0]
	m.responses[req.ModelProfile] = recorders[1:]

	return ai.GenerateResponse{
		Message:  rec.Message,
		Usage:    rec.Usage,
		ModelID:  "mock-model",
		CostUSD:  rec.CostUSD,
	}, nil
}

// Stream returns a mock stream with canned chunks.
func (m *MockClient) Stream(_ context.Context, req ai.GenerateRequest) (ai.StreamReader, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.err != nil {
		return nil, m.err
	}

	chunks, ok := m.streams[req.ModelProfile]
	if !ok || len(chunks) == 0 {
		return nil, fmt.Errorf("aitest: no recorded stream for profile %q", req.ModelProfile)
	}

	return &mockStreamReader{chunks: chunks}, nil
}

// GenerateStructured delegates to Generate and unmarshals.
func (m *MockClient) GenerateStructured(ctx context.Context, req ai.GenerateRequest, out any) error {
	resp, err := m.Generate(ctx, req)
	if err != nil {
		return err
	}
	// For mock, just set the content as-is if out is *string.
	if s, ok := out.(*string); ok {
		*s = resp.Message.Content
	}
	return nil
}

// RunAgent delegates to Generate for simplicity in tests.
func (m *MockClient) RunAgent(ctx context.Context, req ai.AgentRequest) (ai.AgentResponse, error) {
	genReq := ai.GenerateRequest{
		ModelProfile: req.ModelProfile,
		Messages:     req.Messages,
		System:       req.System,
		Temperature:  req.Temperature,
		MaxTokens:    req.MaxTokens,
		Metadata:     req.Metadata,
	}
	resp, err := m.Generate(ctx, genReq)
	if err != nil {
		return ai.AgentResponse{}, err
	}
	return ai.AgentResponse{
		Messages: []ai.Message{resp.Message},
		Usage:    resp.Usage,
		ModelID:  resp.ModelID,
		Steps:    1,
		CostUSD:  resp.CostUSD,
	}, nil
}

// mockStreamReader implements ai.StreamReader with canned chunks.
type mockStreamReader struct {
	chunks []ai.Chunk
	pos    int
}

func (r *mockStreamReader) Recv() (ai.Chunk, error) {
	if r.pos >= len(r.chunks) {
		return ai.Chunk{}, io.EOF
	}
	chunk := r.chunks[r.pos]
	r.pos++
	return chunk, nil
}

func (r *mockStreamReader) Close() {
	r.pos = len(r.chunks)
}

// Ensure MockClient implements ai.Client.
var _ ai.Client = (*MockClient)(nil)
