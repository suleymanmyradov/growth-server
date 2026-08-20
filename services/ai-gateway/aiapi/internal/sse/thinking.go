package sse

import (
	"sync"
	"time"
)

// ThinkingGuard sends periodic "thinking" SSE events while the model
// processes before the first real event (delta, reasoning, tool call)
// arrives. This keeps the connection alive and gives the user feedback
// during the initial LLM call, which can take several seconds.
//
// The guard runs a goroutine that ticks at the configured interval and
// writes "thinking" events via the shared Writer. It stops when:
//   - SignalFirstEvent() is called (first real event arrived), or
//   - Stop() is called (stream ended or errored).
//
// Stop() and SignalFirstEvent() both wait for the goroutine to exit
// before returning, so the caller can safely read the response body
// after either call.
//
// Usage:
//
//	tg := sse.NewThinkingGuard(sseWriter, thinkingMessages, 2*time.Second)
//	tg.Start()
//	defer tg.Stop()
//	// ... in the forward loop, on first real event:
//	tg.SignalFirstEvent()
type ThinkingGuard struct {
	writer   *Writer
	messages []string
	interval time.Duration

	firstEvent chan struct{}
	wg         sync.WaitGroup
	stopOnce   sync.Once
}

// NewThinkingGuard creates a ThinkingGuard that cycles through messages
// at the given interval. Does not start the goroutine — call Start().
func NewThinkingGuard(w *Writer, messages []string, interval time.Duration) *ThinkingGuard {
	return &ThinkingGuard{
		writer:     w,
		messages:   messages,
		interval:   interval,
		firstEvent: make(chan struct{}),
	}
}

// Start launches the thinking goroutine. It is safe to call once.
func (tg *ThinkingGuard) Start() {
	tg.wg.Add(1)
	go tg.run()
}

// run is the goroutine that sends periodic thinking events.
func (tg *ThinkingGuard) run() {
	defer tg.wg.Done()
	ticker := time.NewTicker(tg.interval)
	defer ticker.Stop()
	idx := 0
	for {
		select {
		case <-tg.firstEvent:
			return
		case <-ticker.C:
			msg := tg.messages[idx%len(tg.messages)]
			idx++
			if !tg.writer.WriteEvent("thinking", map[string]string{"message": msg}) {
				return
			}
		}
	}
}

// SignalFirstEvent signals that the first real event has arrived,
// stopping the thinking goroutine and waiting for it to exit.
// Idempotent — safe to call multiple times or after Stop.
func (tg *ThinkingGuard) SignalFirstEvent() {
	tg.stopOnce.Do(func() {
		close(tg.firstEvent)
	})
	tg.wg.Wait()
}

// Stop signals the thinking goroutine to stop and waits for it to exit.
// Idempotent — safe to call multiple times.
func (tg *ThinkingGuard) Stop() {
	tg.stopOnce.Do(func() {
		close(tg.firstEvent)
	})
	tg.wg.Wait()
}

// HeartbeatGuard sends periodic SSE keepalive comments to keep the
// connection alive during long silent periods (e.g. structured JSON
// generation that produces no output for 60+ seconds).
type HeartbeatGuard struct {
	writer   *Writer
	interval time.Duration
	done     chan struct{}
	wg       sync.WaitGroup
}

// NewHeartbeatGuard creates a HeartbeatGuard that sends keepalive
// comments at the given interval.
func NewHeartbeatGuard(w *Writer, interval time.Duration) *HeartbeatGuard {
	return &HeartbeatGuard{
		writer:   w,
		interval: interval,
		done:     make(chan struct{}),
	}
}

// Start launches the heartbeat goroutine.
func (hg *HeartbeatGuard) Start() {
	hg.wg.Add(1)
	go hg.run()
}

func (hg *HeartbeatGuard) run() {
	defer hg.wg.Done()
	ticker := time.NewTicker(hg.interval)
	defer ticker.Stop()
	for {
		select {
		case <-hg.done:
			return
		case <-ticker.C:
			if !hg.writer.WriteKeepalive() {
				return
			}
		}
	}
}

// Stop signals the heartbeat goroutine to stop and waits for it to exit.
// Idempotent — safe to call multiple times (subsequent calls are no-ops
// but will still Wait on the already-completed WaitGroup).
func (hg *HeartbeatGuard) Stop() {
	select {
	case <-hg.done:
	default:
		close(hg.done)
	}
	hg.wg.Wait()
}
