package events

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeDLQ struct {
	mu   sync.Mutex
	msgs []DLQMessage
	err  error
}

func (f *fakeDLQ) Publish(_ context.Context, msg DLQMessage) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.msgs = append(f.msgs, msg)
	return nil
}

func (f *fakeDLQ) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.msgs)
}

func TestWithRetryConsume_SucceedsFirstTry(t *testing.T) {
	dlq := &fakeDLQ{}
	calls := 0
	h := WithRetryConsume(dlq, RetryConsumeConfig{ServiceName: "test"}, func(context.Context, string, string) error {
		calls++
		return nil
	})
	require.NoError(t, h(context.Background(), "k", "v"))
	assert.Equal(t, 1, calls)
	assert.Equal(t, 0, dlq.count())
}

func TestWithRetryConsume_RetriesThenSucceeds(t *testing.T) {
	dlq := &fakeDLQ{}
	calls := 0
	h := WithRetryConsume(dlq, RetryConsumeConfig{
		MaxAttempts:    4,
		InitialBackoff: time.Millisecond,
		MaxBackoff:     2 * time.Millisecond,
		ServiceName:    "test",
	}, func(context.Context, string, string) error {
		calls++
		if calls < 3 {
			return errors.New("transient")
		}
		return nil
	})
	require.NoError(t, h(context.Background(), "k", "v"))
	assert.Equal(t, 3, calls)
	assert.Equal(t, 0, dlq.count())
}

func TestWithRetryConsume_ExhaustedGoesToDLQ(t *testing.T) {
	dlq := &fakeDLQ{}
	calls := 0
	h := WithRetryConsume(dlq, RetryConsumeConfig{
		MaxAttempts:    3,
		InitialBackoff: time.Millisecond,
		ServiceName:    "test-svc",
	}, func(context.Context, string, string) error {
		calls++
		return errors.New("always fails")
	})
	// After exhausting attempts the wrapper returns nil so the offset commits —
	// the failure is preserved in the DLQ instead of silently dropped.
	require.NoError(t, h(context.Background(), "k", "raw-message"))
	assert.Equal(t, 3, calls)
	require.Equal(t, 1, dlq.count())
	assert.Equal(t, "raw-message", dlq.msgs[0].Raw)
	assert.Equal(t, "test-svc", dlq.msgs[0].ServiceName)
	assert.Equal(t, "always fails", dlq.msgs[0].Reason)
	assert.False(t, dlq.msgs[0].Permanent)
}

func TestWithRetryConsume_DLQFailureReturnsError(t *testing.T) {
	dlq := &fakeDLQ{err: errors.New("dlq down")}
	h := WithRetryConsume(dlq, RetryConsumeConfig{
		MaxAttempts:    2,
		InitialBackoff: time.Millisecond,
	}, func(context.Context, string, string) error {
		return errors.New("handler failed")
	})
	err := h(context.Background(), "k", "v")
	require.Error(t, err)
	assert.Equal(t, "handler failed", err.Error())
}

func TestWithRetryConsume_NoDLQStillStops(t *testing.T) {
	calls := 0
	h := WithRetryConsume(nil, RetryConsumeConfig{
		MaxAttempts:    2,
		InitialBackoff: time.Millisecond,
	}, func(context.Context, string, string) error {
		calls++
		return errors.New("boom")
	})
	require.NoError(t, h(context.Background(), "k", "v"))
	assert.Equal(t, 2, calls)
}
