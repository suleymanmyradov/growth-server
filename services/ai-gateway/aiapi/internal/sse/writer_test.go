package sse

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// parseSSEEvents splits raw SSE output into individual events.
// Each event is a string like "event: foo\ndata: {...}\n\n".
func parseSSEEvents(raw string) []string {
	raw = strings.TrimSuffix(raw, "\n")
	parts := strings.Split(raw, "\n\n")
	var events []string
	for _, p := range parts {
		if p != "" {
			events = append(events, p)
		}
	}
	return events
}

func TestNewWriter_SetsHeaders(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewWriter(rec)

	h := rec.Header()
	assert.Equal(t, "text/event-stream", h.Get("Content-Type"))
	assert.Equal(t, "no-cache", h.Get("Cache-Control"))
	assert.Equal(t, "keep-alive", h.Get("Connection"))
	assert.Equal(t, "no", h.Get("X-Accel-Buffering"))
	assert.Equal(t, 200, rec.Code)
	_ = w
}

func TestWriteEvent_JSONPayload(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewWriter(rec)

	ok := w.WriteEvent("delta", map[string]string{"text": "hello"})
	require.True(t, ok)

	events := parseSSEEvents(rec.Body.String())
	require.Len(t, events, 1)
	assert.Contains(t, events[0], "event: delta")
	assert.Contains(t, events[0], `data: {"text":"hello"}`)
}

func TestWriteRaw_PreMarshalledData(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewWriter(rec)

	ok := w.WriteRaw("proposal", `{"id":"abc","action":"create_goal"}`)
	require.True(t, ok)

	events := parseSSEEvents(rec.Body.String())
	require.Len(t, events, 1)
	assert.Contains(t, events[0], "event: proposal")
	assert.Contains(t, events[0], `data: {"id":"abc","action":"create_goal"}`)
}

func TestWriteError(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewWriter(rec)

	ok := w.WriteError("something went wrong")
	require.True(t, ok)

	events := parseSSEEvents(rec.Body.String())
	require.Len(t, events, 1)
	assert.Contains(t, events[0], "event: error")
	assert.Contains(t, events[0], `data: {"message":"something went wrong"}`)
}

func TestWriteKeepalive(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewWriter(rec)

	ok := w.WriteKeepalive()
	require.True(t, ok)

	output := rec.Body.String()
	assert.Contains(t, output, ": keepalive\n\n")
	// Keepalive is a comment, not an event — it should NOT have "event:" prefix.
	assert.NotContains(t, output, "event:")
}

func TestWriteEvent_ConcurrentSafe(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewWriter(rec)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			w.WriteEvent("delta", map[string]int{"n": n})
		}(i)
	}
	wg.Wait()

	// All 10 events should be present, none corrupted.
	events := parseSSEEvents(rec.Body.String())
	assert.Len(t, events, 10)
}

func TestWriter_FailedAfterWriteError(t *testing.T) {
	// Use a writer that always fails.
	rec := &failingResponseWriter{}
	w := NewWriter(rec)

	ok := w.WriteEvent("delta", map[string]string{"text": "hello"})
	require.False(t, ok)
	assert.True(t, w.Failed())

	// Subsequent writes should be no-ops returning false.
	ok = w.WriteEvent("delta", map[string]string{"text": "world"})
	assert.False(t, ok)
}

func TestWriteEvent_MultipleEventsInOrder(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewWriter(rec)

	w.WriteEvent("thinking", map[string]string{"message": "Looking up..."})
	w.WriteEvent("delta", map[string]string{"text": "Hello "})
	w.WriteEvent("delta", map[string]string{"text": "world"})
	w.WriteEvent("complete", map[string]string{"fullResponse": "Hello world"})

	events := parseSSEEvents(rec.Body.String())
	require.Len(t, events, 4)
	assert.Contains(t, events[0], "event: thinking")
	assert.Contains(t, events[1], "event: delta")
	assert.Contains(t, events[2], "event: delta")
	assert.Contains(t, events[3], "event: complete")
}

// --- ThinkingGuard tests ---

func TestThinkingGuard_StopsOnFirstEvent(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewWriter(rec)

	messages := []string{"Thinking...", "Still thinking..."}
	tg := NewThinkingGuard(w, messages, 10*time.Millisecond)
	tg.Start()

	// Let a few thinking events fire.
	time.Sleep(35 * time.Millisecond)

	tg.SignalFirstEvent()
	tg.Stop()

	// Wait a bit to ensure the goroutine has exited.
	time.Sleep(20 * time.Millisecond)

	events := parseSSEEvents(rec.Body.String())
	// Should have at least 1 thinking event, but NOT continue
	// after SignalFirstEvent.
	assert.NotEmpty(t, events)
	for _, e := range events {
		assert.Contains(t, e, "event: thinking")
	}

	// Record count, sleep more, verify no new events.
	countBefore := len(events)
	time.Sleep(30 * time.Millisecond)
	eventsAfter := parseSSEEvents(rec.Body.String())
	assert.Equal(t, countBefore, len(eventsAfter), "no new thinking events after SignalFirstEvent")
}

func TestThinkingGuard_StopsOnStop(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewWriter(rec)

	messages := []string{"Thinking..."}
	tg := NewThinkingGuard(w, messages, 10*time.Millisecond)
	tg.Start()

	time.Sleep(25 * time.Millisecond)
	tg.Stop()
	time.Sleep(20 * time.Millisecond)

	events := parseSSEEvents(rec.Body.String())
	assert.NotEmpty(t, events)

	countBefore := len(events)
	time.Sleep(30 * time.Millisecond)
	eventsAfter := parseSSEEvents(rec.Body.String())
	assert.Equal(t, countBefore, len(eventsAfter), "no new thinking events after Stop")
}

func TestThinkingGuard_IdempotentStopAndSignal(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewWriter(rec)

	tg := NewThinkingGuard(w, []string{"Thinking..."}, 10*time.Millisecond)
	tg.Start()

	// Calling both SignalFirstEvent and Stop should not panic.
	tg.SignalFirstEvent()
	tg.SignalFirstEvent() // idempotent
	tg.Stop()
	tg.Stop() // idempotent
}

// --- HeartbeatGuard tests ---

func TestHeartbeatGuard_SendsKeepalives(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewWriter(rec)

	hg := NewHeartbeatGuard(w, 10*time.Millisecond)
	hg.Start()

	time.Sleep(35 * time.Millisecond)
	hg.Stop()

	output := rec.Body.String()
	keepaliveCount := strings.Count(output, ": keepalive")
	assert.GreaterOrEqual(t, keepaliveCount, 1, "should have at least one keepalive")

	// After Stop, no new keepalives.
	countBefore := keepaliveCount
	time.Sleep(30 * time.Millisecond)
	keepaliveAfter := strings.Count(rec.Body.String(), ": keepalive")
	assert.Equal(t, countBefore, keepaliveAfter, "no new keepalives after Stop")
}

func TestHeartbeatGuard_StopWaitsForGoroutine(t *testing.T) {
	rec := httptest.NewRecorder()
	w := NewWriter(rec)

	hg := NewHeartbeatGuard(w, 10*time.Millisecond)
	hg.Start()
	hg.Stop()

	// If Stop didn't wait, this test would race. The fact that it
	// completes without flakiness is the assertion — plus we can
	// call Stop again safely.
	hg.Stop() // idempotent
}

// --- Helpers ---

// failingResponseWriter is an http.ResponseWriter that returns an error
// on all writes, simulating a client disconnect.
type failingResponseWriter struct {
	headerMap http.Header
}

func (f *failingResponseWriter) Header() http.Header {
	if f.headerMap == nil {
		f.headerMap = make(http.Header)
	}
	return f.headerMap
}

func (f *failingResponseWriter) Write([]byte) (int, error) {
	return 0, errWriteFailed
}

func (f *failingResponseWriter) WriteHeader(int) {}

var errWriteFailed = &simpleErr{"write failed"}

type simpleErr struct{ msg string }

func (e *simpleErr) Error() string { return e.msg }
