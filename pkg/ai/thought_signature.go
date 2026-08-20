package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

// Gemini thinking models attach a thought_signature to every function call
// part. When the conversation history is sent back (stateless mode), Gemini
// requires each functionCall part to carry its original thought_signature —
// otherwise it rejects the request with 400 INVALID_ARGUMENT:
//
//	"Function call is missing a thought_signature in functionCall parts."
//
// The signature travels inside a Gemini-specific extension to the OpenAI
// chat completion format:
//
//	"tool_calls": [{
//	  "id": "call_123",
//	  "type": "function",
//	  "function": {"name": "...", "arguments": "..."},
//	  "extra_content": {"google": {"thought_signature": "base64..."}}
//	}]
//
// The go-openai and eino-ext libraries do not model extra_content on
// ToolCall, so the signature is silently dropped during JSON unmarshaling.
// This file implements a transport-level workaround that:
//
//  1. Captures thought_signature values from the streaming (SSE) and
//     non-streaming JSON response body without modifying the bytes the
//     upstream libraries receive.
//  2. Injects the captured signatures back into assistant tool_calls on
//     subsequent requests so Gemini accepts the conversation history.
//
// The capture/inject state is passed through context so the transport is
// a no-op for non-agent calls (Generate, Stream) that don't opt in.

// --- Context keys ---

type captureCtxKey struct{}
type injectCtxKey struct{}

// withCapture returns a context carrying a thoughtSignatureCapture. The
// transport reads it from the HTTP request context and populates it while
// reading the response body.
func withCapture(ctx context.Context, c *thoughtSignatureCapture) context.Context {
	return context.WithValue(ctx, captureCtxKey{}, c)
}

// withInject returns a context carrying a toolCallID→signature map. The
// transport reads it from the HTTP request context and injects each
// signature into the matching tool_call in the request body.
func withInject(ctx context.Context, sigs map[string]string) context.Context {
	return context.WithValue(ctx, injectCtxKey{}, sigs)
}

// captureFromContext returns the capture stored in ctx, or nil.
func captureFromContext(ctx context.Context) *thoughtSignatureCapture {
	c, _ := ctx.Value(captureCtxKey{}).(*thoughtSignatureCapture)
	return c
}

// injectFromContext returns the injection map stored in ctx, or nil.
func injectFromContext(ctx context.Context) map[string]string {
	m, _ := ctx.Value(injectCtxKey{}).(map[string]string)
	return m
}

// --- Capture ---

// thoughtSignatureCapture collects thought_signature values from a single
// model response, keyed by tool call ID. It is safe for concurrent use.
type thoughtSignatureCapture struct {
	mu   sync.Mutex
	sigs map[string]string // toolCallID → thought_signature
}

func newThoughtSignatureCapture() *thoughtSignatureCapture {
	return &thoughtSignatureCapture{sigs: make(map[string]string)}
}

func (c *thoughtSignatureCapture) add(toolCallID, signature string) {
	if toolCallID == "" || signature == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sigs[toolCallID] = signature
}

// get returns the signature for the given tool call ID, or "".
func (c *thoughtSignatureCapture) get(toolCallID string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.sigs[toolCallID]
}

// all returns a copy of all captured signatures.
func (c *thoughtSignatureCapture) all() map[string]string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[string]string, len(c.sigs))
	for k, v := range c.sigs {
		out[k] = v
	}
	return out
}

// --- Transport ---

// thoughtSignatureTransport wraps an http.RoundTripper to capture
// thought_signature values from Gemini responses and inject them into
// subsequent requests. It is a no-op when the request context does not
// carry a capture or inject value.
type thoughtSignatureTransport struct {
	base http.RoundTripper
}

func (t *thoughtSignatureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// --- Request side: inject signatures into assistant tool_calls ---
	if sigs := injectFromContext(req.Context()); len(sigs) > 0 {
		if req.Body != nil {
			body, err := io.ReadAll(req.Body)
			_ = req.Body.Close()
			if err == nil {
				if patched, pErr := injectThoughtSignatures(body, sigs); pErr == nil {
					req.Body = io.NopCloser(bytes.NewReader(patched))
					req.ContentLength = int64(len(patched))
				} else {
					req.Body = io.NopCloser(bytes.NewReader(body))
					req.ContentLength = int64(len(body))
				}
			}
		}
	}

	resp, err := t.base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	// --- Error response normalization: unwrap array-wrapped errors ---
	// Google's Gemini OpenAI-compat endpoint sometimes returns errors as
	// a JSON array ([{"error":{...}}]) instead of the standard object
	// form ({"error":{...}}). The go-openai client can't unmarshal arrays
	// into its ErrorResponse struct, so the real error (status code,
	// message) is masked behind a JSON parse error. Normalize the body
	// before the upstream client sees it. This applies to ALL error
	// responses, not just agent calls with capture context.
	if resp.StatusCode >= 400 {
		resp = normalizeErrorResponse(resp)
	}

	// --- Response side: capture signatures from the response body ---
	capture := captureFromContext(req.Context())
	if capture == nil || resp.Body == nil {
		return resp, nil
	}

	ct := resp.Header.Get("Content-Type")
	if isSSEContentType(ct) {
		resp.Body = newSSESignatureReader(resp.Body, capture)
	} else if isJSONContentType(ct) {
		resp.Body = newJSONSignatureReader(resp.Body, capture)
	}

	return resp, nil
}

func isSSEContentType(ct string) bool {
	return containsFold(ct, "text/event-stream")
}

func isJSONContentType(ct string) bool {
	return containsFold(ct, "application/json")
}

func containsFold(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || foldContains(s, substr))
}

func foldContains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca == cb {
			continue
		}
		if 'A' <= ca && ca <= 'Z' {
			ca += 32
		}
		if 'A' <= cb && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// --- Request injection ---

// injectThoughtSignatures patches a chat completion request body to add
// extra_content.google.thought_signature to assistant tool_calls whose ID
// is in the signatures map. It preserves all other fields.
func injectThoughtSignatures(body []byte, signatures map[string]string) ([]byte, error) {
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	messages, ok := req["messages"].([]any)
	if !ok {
		return body, nil
	}

	for _, msg := range messages {
		m, ok := msg.(map[string]any)
		if !ok {
			continue
		}
		if m["role"] != "assistant" {
			continue
		}
		toolCalls, ok := m["tool_calls"].([]any)
		if !ok {
			continue
		}
		for _, tc := range toolCalls {
			tcm, ok := tc.(map[string]any)
			if !ok {
				continue
			}
			id, _ := tcm["id"].(string)
			sig, ok := signatures[id]
			if !ok {
				continue
			}
			tcm["extra_content"] = map[string]any{
				"google": map[string]any{
					"thought_signature": sig,
				},
			}
		}
	}

	return json.Marshal(req)
}

// --- SSE response reader (streaming) ---

// sseSignatureReader wraps an SSE response body, extracting
// thought_signature values from data: lines as they pass through. The
// original bytes are returned unchanged so the upstream library can parse
// them normally.
type sseSignatureReader struct {
	src     io.ReadCloser
	reader  *bufio.Reader
	capture *thoughtSignatureCapture
	pending bytes.Buffer
}

func newSSESignatureReader(src io.ReadCloser, capture *thoughtSignatureCapture) *sseSignatureReader {
	return &sseSignatureReader{
		src:     src,
		reader:  bufio.NewReader(src),
		capture: capture,
	}
}

func (r *sseSignatureReader) Read(p []byte) (int, error) {
	if r.pending.Len() > 0 {
		return r.pending.Read(p)
	}

	line, err := r.reader.ReadBytes('\n')
	if len(line) > 0 {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("data: ")) {
			data := trimmed[len("data: "):]
			if !bytes.Equal(data, []byte("[DONE]")) {
				r.extractSignatures(data)
			}
		}
		r.pending.Write(line)
		return r.pending.Read(p)
	}

	if err != nil {
		return 0, err
	}
	return 0, nil
}

func (r *sseSignatureReader) Close() error {
	return r.src.Close()
}

func (r *sseSignatureReader) extractSignatures(data []byte) {
	var resp struct {
		Choices []struct {
			Delta struct {
				ToolCalls []struct {
					ID           string `json:"id"`
					Index        *int   `json:"index"`
					ExtraContent struct {
						Google struct {
							ThoughtSignature string `json:"thought_signature"`
						} `json:"google"`
					} `json:"extra_content"`
				} `json:"tool_calls"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return
	}
	for _, choice := range resp.Choices {
		for _, tc := range choice.Delta.ToolCalls {
			sig := tc.ExtraContent.Google.ThoughtSignature
			if sig == "" {
				continue
			}
			key := tc.ID
			if key == "" && tc.Index != nil {
				key = fmt.Sprintf("index_%d", *tc.Index)
			}
			r.capture.add(key, sig)
		}
	}
}

// --- JSON response reader (non-streaming) ---

// jsonSignatureReader reads the entire response body, extracts
// thought_signature values from the non-streaming JSON response, then
// returns the original bytes via a bytes.Reader.
type jsonSignatureReader struct {
	body *bytes.Reader
	src  io.ReadCloser
	read bool
}

func newJSONSignatureReader(src io.ReadCloser, capture *thoughtSignatureCapture) *jsonSignatureReader {
	r := &jsonSignatureReader{src: src}
	data, err := io.ReadAll(src)
	if err == nil {
		r.extractSignatures(data, capture)
	}
	r.body = bytes.NewReader(data)
	r.read = true
	return r
}

func (r *jsonSignatureReader) Read(p []byte) (int, error) {
	if !r.read {
		// Fallback: shouldn't happen since constructor reads eagerly.
		return r.src.Read(p)
	}
	return r.body.Read(p)
}

func (r *jsonSignatureReader) Close() error {
	return r.src.Close()
}

func (r *jsonSignatureReader) extractSignatures(data []byte, capture *thoughtSignatureCapture) {
	var resp struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					ID           string `json:"id"`
					ExtraContent struct {
						Google struct {
							ThoughtSignature string `json:"thought_signature"`
						} `json:"google"`
					} `json:"extra_content"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return
	}
	for _, choice := range resp.Choices {
		for _, tc := range choice.Message.ToolCalls {
			if tc.ExtraContent.Google.ThoughtSignature != "" {
				capture.add(tc.ID, tc.ExtraContent.Google.ThoughtSignature)
			}
		}
	}
}
