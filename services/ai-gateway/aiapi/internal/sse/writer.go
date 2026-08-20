package sse

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
)

// Writer is a thread-safe SSE writer for HTTP streaming responses.
// It serializes all writes through a mutex so concurrent goroutines
// (thinking heartbeats, main forward loops) cannot interleave or
// corrupt SSE frames.
//
// The zero value is not usable — use NewWriter.
type Writer struct {
	w       http.ResponseWriter
	flusher http.Flusher
	mu      sync.Mutex
	failed  bool
}

// NewWriter commits SSE headers and a 200 status on the response writer,
// then returns a Writer ready to stream events. Must be called exactly
// once per response, before any writes.
func NewWriter(w http.ResponseWriter) *Writer {
	setSSEHeaders(w)
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	return &Writer{
		w:       w,
		flusher: flusher,
	}
}

// setSSEHeaders sets the standard SSE headers on the response writer.
// Exported via NewWriter; also used directly by handlers that need to
// commit headers before constructing a Writer (e.g. cached-response
// short-circuits).
func setSSEHeaders(w http.ResponseWriter) {
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("Cache-Control", "no-cache")
	h.Set("Connection", "keep-alive")
	h.Set("X-Accel-Buffering", "no")
}

// WriteEvent writes an SSE event with a JSON-marshaled payload.
// Returns false if the write failed (client disconnect or write error);
// the caller should stop writing and return.
func (sw *Writer) WriteEvent(event string, payload any) bool {
	data, err := json.Marshal(payload)
	if err != nil {
		return false
	}
	return sw.WriteRaw(event, string(data))
}

// WriteRaw writes an SSE event with pre-marshaled data.
// Returns false if the write failed.
func (sw *Writer) WriteRaw(event, data string) bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.failed {
		return false
	}
	if _, err := fmt.Fprintf(sw.w, "event: %s\ndata: %s\n\n", event, data); err != nil {
		sw.failed = true
		return false
	}
	sw.flush()
	return true
}

// WriteError writes an error SSE event with a JSON {"message": msg} payload.
func (sw *Writer) WriteError(msg string) bool {
	return sw.WriteEvent("error", map[string]string{"message": msg})
}

// WriteKeepalive writes an SSE comment line (": keepalive\n\n") to keep
// the connection alive during long silent periods (e.g. structured JSON
// generation that can take 60+ seconds with no output).
func (sw *Writer) WriteKeepalive() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	if sw.failed {
		return false
	}
	if _, err := fmt.Fprintf(sw.w, ": keepalive\n\n"); err != nil {
		sw.failed = true
		return false
	}
	sw.flush()
	return true
}

// Failed returns true if a previous write failed (client disconnect).
// Once true, all subsequent writes are no-ops.
func (sw *Writer) Failed() bool {
	sw.mu.Lock()
	defer sw.mu.Unlock()
	return sw.failed
}

// flush flushes the underlying ResponseWriter if it supports flushing.
// Caller must hold sw.mu.
func (sw *Writer) flush() {
	if sw.flusher != nil {
		sw.flusher.Flush()
	}
}
