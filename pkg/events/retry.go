package events

import (
	"context"
	"time"

	"github.com/zeromicro/go-queue/kq"
	"github.com/zeromicro/go-zero/core/logx"
)

// ConsumeFunc is the message-handler signature shared by the kq (Kafka) and
// redisstream transports: Consume(ctx, key, value).
type ConsumeFunc func(ctx context.Context, key, value string) error

// Consume lets a ConsumeFunc satisfy the redisstream.ConsumeHandler
// interface (and any other Consume(ctx, key, value) interface).
func (f ConsumeFunc) Consume(ctx context.Context, key, value string) error {
	return f(ctx, key, value)
}

// Handle converts a ConsumeFunc to the kq.ConsumeHandle type expected by
// kq.WithHandle.
func (f ConsumeFunc) Handle() kq.ConsumeHandle {
	return kq.ConsumeHandle(f)
}

// dlqSink is the subset of DLQPublisher needed by WithRetryConsume — an
// interface so tests can inject a fake.
type dlqSink interface {
	Publish(ctx context.Context, msg DLQMessage) error
}

// RetryConsumeConfig tunes WithRetryConsume. Zero fields get defaults.
type RetryConsumeConfig struct {
	// MaxAttempts is the total number of handler invocations, including the
	// first. Default 5.
	MaxAttempts int
	// InitialBackoff is the delay before the first retry. Default 100ms.
	InitialBackoff time.Duration
	// MaxBackoff caps the exponential backoff between attempts. Default 2s.
	MaxBackoff time.Duration
	// ServiceName is stamped onto DLQ messages so poison events are
	// attributable. Default "unknown".
	ServiceName string
}

// WithRetryConsume wraps fn with bounded retries and a DLQ fallback.
//
// Retries must happen inside the handler: kq auto-commits every fetched
// offset on a fixed interval, so a returned error is logged but the message
// is committed anyway and never redelivered — a handler error would silently
// drop the event without this wrapper. After MaxAttempts the raw message is
// published to the DLQ (visible, replayable) and nil is returned so the
// offset commits. If the DLQ publish itself fails, the original error is
// returned so the failure is at least surfaced by kq's errorHandler/metrics.
//
// The wrapped handler must stay idempotent — retries and DLQ replays mean it
// can be invoked more than once for the same event.
func WithRetryConsume(dlq dlqSink, cfg RetryConsumeConfig, fn ConsumeFunc) ConsumeFunc {
	maxAttempts := cfg.MaxAttempts
	if maxAttempts <= 0 {
		maxAttempts = 5
	}
	initial := cfg.InitialBackoff
	if initial <= 0 {
		initial = 100 * time.Millisecond
	}
	maxBackoff := cfg.MaxBackoff
	if maxBackoff <= 0 {
		maxBackoff = 2 * time.Second
	}
	service := cfg.ServiceName
	if service == "" {
		service = "unknown"
	}

	return func(ctx context.Context, key, value string) error {
		var err error
		backoff := initial
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			if err = fn(ctx, key, value); err == nil {
				return nil
			}
			if attempt == maxAttempts {
				break
			}
			logx.WithContext(ctx).Errorf(
				"events: %s consume attempt %d/%d failed, retrying: %v",
				service, attempt, maxAttempts, err)
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
		}

		logx.WithContext(ctx).Errorf(
			"events: %s consume failed after %d attempts, sending to DLQ: %v",
			service, maxAttempts, err)
		if dlq != nil {
			if pubErr := dlq.Publish(ctx, DLQMessage{
				Raw:         value,
				Reason:      err.Error(),
				Permanent:   false,
				ServiceName: service,
				OccurredAt:  time.Now().UTC(),
			}); pubErr != nil {
				logx.WithContext(ctx).Errorf(
					"events: %s DLQ publish failed: %v (original error: %v)",
					service, pubErr, err)
				return err
			}
		}
		return nil
	}
}
