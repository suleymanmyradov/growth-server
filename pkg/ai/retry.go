package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/cenkalti/backoff/v5"
	openaimodel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/zeromicro/go-zero/core/logx"
)

// retryFn is the function signature for operations that can be retried.
type retryFn func(ctx context.Context) error

// withRetry wraps an operation with exponential backoff, per-attempt timeouts,
// and circuit breaker. Retryable errors (429/5xx) are retried without tripping
// the breaker; non-retryable errors count as breaker failures.
func (c *client) withRetry(ctx context.Context, modelID string, fn retryFn) error {
	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = c.cfg.RetryBackoff
	eb.MaxInterval = 30 * time.Second

	attempt := 0
	operation := func() (struct{}, error) {
		attempt++

		// Per-attempt timeout: isolate each attempt so a slow first call
		// doesn't starve retries of their own deadline budget.
		perAttemptTimeout := c.cfg.DefaultTimeout
		if deadline, ok := ctx.Deadline(); ok {
			remaining := time.Until(deadline)
			if remaining > 0 && remaining < perAttemptTimeout {
				perAttemptTimeout = remaining
			}
		}
		attemptCtx, cancel := context.WithTimeout(ctx, perAttemptTimeout)
		defer cancel()

		// Always run through the circuit breaker. The acceptable function
		// tells the breaker to ignore retryable errors (they don't count as
		// failures) so the breaker only tracks non-retryable errors and
		// successes. When the breaker is open, DoWithAcceptable returns
		// ErrServiceUnavailable immediately.
		brk := c.breakerFor(modelID)
		var lastErr error
		brkErr := brk.DoWithAcceptable(func() error {
			lastErr = fn(attemptCtx)
			return lastErr
		}, func(err error) bool {
			if err == nil {
				return true
			}
			// Retryable errors are "acceptable" — the breaker should NOT
			// count them as failures.
			return isRetryable(err)
		})

		if brkErr != nil {
			// Breaker is open.
			logx.WithContext(ctx).Infof("ai: circuit breaker open for model %s: %v", modelID, brkErr)
			return struct{}{}, backoff.Permanent(fmt.Errorf("ai: circuit breaker: %w", brkErr))
		}

		if lastErr != nil {
			// Acceptable retryable error: trigger backoff retry.
			logx.WithContext(ctx).Infof("ai: retryable error on model %s (attempt %d): %v", modelID, attempt, lastErr)
			return struct{}{}, lastErr
		}

		return struct{}{}, nil
	}

	_, err := backoff.Retry(ctx, operation,
		backoff.WithBackOff(eb),
		backoff.WithMaxTries(uint(c.cfg.MaxRetries)),
	)
	if err != nil {
		return fmt.Errorf("ai: retries exhausted for model %s: %w", modelID, err)
	}
	return nil
}

// isRetryable returns true for 429 and 5xx errors.
func isRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for Eino OpenAI adapter's APIError type.
	var einoAPIErr *openaimodel.APIError
	if errors.As(err, &einoAPIErr) {
		code := einoAPIErr.HTTPStatusCode
		return code == http.StatusTooManyRequests || code >= 500
	}

	// Check for our internal apiError type.
	if apiErr, ok := unwrapAPIError(err); ok {
		code := apiErr.StatusCode
		return code == http.StatusTooManyRequests || code >= 500
	}

	// Context errors are not retryable.
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}

	// Default: retry on unknown errors (network issues, etc.).
	return true
}

// apiError represents an HTTP API error with a status code.
type apiError struct {
	StatusCode int
	Message    string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("api error %d: %s", e.StatusCode, e.Message)
}

// unwrapAPIError attempts to extract an apiError from wrapped errors.
func unwrapAPIError(err error) (*apiError, bool) {
	for err != nil {
		if ae, ok := err.(*apiError); ok {
			return ae, true
		}
		if wrapper, ok := err.(interface{ Unwrap() error }); ok {
			err = wrapper.Unwrap()
		} else {
			break
		}
	}
	return nil, false
}
