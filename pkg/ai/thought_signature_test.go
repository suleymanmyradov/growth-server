package ai

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

// staticTransport is a test RoundTripper that returns a fixed response.
type staticTransport struct {
	status   int
	headers  map[string]string
	body     string
	lastReq  *http.Request
	lastBody []byte
}

func (t *staticTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body != nil {
		t.lastReq = req
		t.lastBody, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
		req.Body = io.NopCloser(bytes.NewReader(t.lastBody))
	}
	resp := &http.Response{
		StatusCode: t.status,
		Body:       io.NopCloser(strings.NewReader(t.body)),
		Header:     make(http.Header),
	}
	for k, v := range t.headers {
		resp.Header.Set(k, v)
	}
	return resp, nil
}

func TestInjectThoughtSignatures(t *testing.T) {
	body := `{"model":"gemini-flash-latest","messages":[
		{"role":"user","content":"hi"},
		{"role":"assistant","content":"","tool_calls":[
			{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Paris\"}"}},
			{"id":"call_2","type":"function","function":{"name":"get_time","arguments":"{}"}}
		]},
		{"role":"tool","tool_call_id":"call_1","content":"sunny"},
		{"role":"tool","tool_call_id":"call_2","content":"noon"}
	]}`

	sigs := map[string]string{
		"call_1": "sig_abc",
		"call_2": "sig_def",
	}

	patched, err := injectThoughtSignatures([]byte(body), sigs)
	if err != nil {
		t.Fatalf("injectThoughtSignatures: %v", err)
	}

	// Both tool calls should have extra_content.google.thought_signature.
	if !bytes.Contains(patched, []byte(`"thought_signature":"sig_abc"`)) {
		t.Error("missing thought_signature for call_1")
	}
	if !bytes.Contains(patched, []byte(`"thought_signature":"sig_def"`)) {
		t.Error("missing thought_signature for call_2")
	}
	// Original fields should be preserved.
	if !bytes.Contains(patched, []byte(`"get_weather"`)) {
		t.Error("function name lost during injection")
	}
	if !bytes.Contains(patched, []byte(`"sunny"`)) {
		t.Error("tool result content lost during injection")
	}
}

func TestInjectThoughtSignatures_PartialMatch(t *testing.T) {
	body := `{"messages":[
		{"role":"assistant","tool_calls":[
			{"id":"call_1","type":"function","function":{"name":"foo","arguments":"{}"}},
			{"id":"call_2","type":"function","function":{"name":"bar","arguments":"{}"}}
		]}
	]}`

	sigs := map[string]string{"call_1": "sig_only_first"}

	patched, err := injectThoughtSignatures([]byte(body), sigs)
	if err != nil {
		t.Fatalf("injectThoughtSignatures: %v", err)
	}

	if !bytes.Contains(patched, []byte(`"thought_signature":"sig_only_first"`)) {
		t.Error("missing thought_signature for call_1")
	}
	// call_2 should NOT have a thought_signature.
	if bytes.Contains(patched, []byte(`"sig_for_call_2"`)) {
		t.Error("unexpected thought_signature for call_2")
	}
}

func TestSSESignatureReader(t *testing.T) {
	sseBody := strings.Join([]string{
		`data: {"choices":[{"delta":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"get_weather","arguments":"{\"city\":\"Paris\"}"},"extra_content":{"google":{"thought_signature":"sig_abc"}}}]}}]}`,
		"",
		`data: {"choices":[{"delta":{"role":"assistant"},"finish_reason":"stop"}]}`,
		"",
		`data: [DONE]`,
		"",
	}, "\n")

	capture := newThoughtSignatureCapture()
	reader := newSSESignatureReader(io.NopCloser(strings.NewReader(sseBody)), capture)

	// Read all bytes — they should match the original.
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(out) != sseBody {
		t.Error("SSE body was modified by the signature reader")
	}

	// The signature should have been captured.
	if sig := capture.get("call_1"); sig != "sig_abc" {
		t.Errorf("expected sig_abc, got %q", sig)
	}
}

func TestJSONSignatureReader(t *testing.T) {
	jsonBody := `{"choices":[{"message":{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"foo","arguments":"{}"},"extra_content":{"google":{"thought_signature":"sig_xyz"}}}]}}]}`

	capture := newThoughtSignatureCapture()
	reader := newJSONSignatureReader(io.NopCloser(strings.NewReader(jsonBody)), capture)

	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if string(out) != jsonBody {
		t.Error("JSON body was modified by the signature reader")
	}

	if sig := capture.get("call_1"); sig != "sig_xyz" {
		t.Errorf("expected sig_xyz, got %q", sig)
	}
}

func TestThoughtSignatureTransport_CaptureSSE(t *testing.T) {
	sseBody := strings.Join([]string{
		`data: {"choices":[{"delta":{"tool_calls":[{"id":"call_42","type":"function","function":{"name":"f","arguments":"{}"},"extra_content":{"google":{"thought_signature":"sig_capture"}}}]}}]}`,
		"",
		`data: [DONE]`,
		"",
	}, "\n")

	base := &staticTransport{
		status:  200,
		headers: map[string]string{"Content-Type": "text/event-stream"},
		body:    sseBody,
	}
	transport := &thoughtSignatureTransport{base: base}

	capture := newThoughtSignatureCapture()
	ctx := withCapture(context.Background(), capture)

	req, _ := http.NewRequestWithContext(ctx, "POST", "http://test", bytes.NewReader([]byte(`{}`)))
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read the response body to trigger the capture.
	_, _ = io.ReadAll(resp.Body)

	if sig := capture.get("call_42"); sig != "sig_capture" {
		t.Errorf("expected sig_capture, got %q", sig)
	}
}

func TestThoughtSignatureTransport_InjectRequest(t *testing.T) {
	base := &staticTransport{
		status:  200,
		headers: map[string]string{"Content-Type": "application/json"},
		body:    `{}`,
	}
	transport := &thoughtSignatureTransport{base: base}

	sigs := map[string]string{"call_99": "sig_inject"}
	ctx := withInject(context.Background(), sigs)

	reqBody := `{"messages":[{"role":"assistant","tool_calls":[{"id":"call_99","type":"function","function":{"name":"f","arguments":"{}"}}]}]}`
	req, _ := http.NewRequestWithContext(ctx, "POST", "http://test", bytes.NewReader([]byte(reqBody)))
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if !bytes.Contains(base.lastBody, []byte(`"thought_signature":"sig_inject"`)) {
		t.Error("request body was not patched with thought_signature")
	}
}

func TestThoughtSignatureTransport_NoOpWithoutContext(t *testing.T) {
	base := &staticTransport{
		status:  200,
		headers: map[string]string{"Content-Type": "application/json"},
		body:    `{"ok":true}`,
	}
	transport := &thoughtSignatureTransport{base: base}

	// No capture or inject in context — should be a pure pass-through.
	reqBody := `{"messages":[{"role":"user","content":"hi"}]}`
	req, _ := http.NewRequestWithContext(context.Background(), "POST", "http://test", bytes.NewReader([]byte(reqBody)))
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	out, _ := io.ReadAll(resp.Body)
	if string(out) != `{"ok":true}` {
		t.Errorf("pass-through failed: got %q", out)
	}
	// Request body should be unchanged.
	if string(base.lastBody) != reqBody {
		t.Error("request body was modified without inject context")
	}
}
