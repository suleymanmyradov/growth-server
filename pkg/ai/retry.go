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

// withRetry wraps an operation with exponential backoff and circuit breaker.
func (c *client) withRetry(ctx context.Context, modelID string, fn retryFn) error {
	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = c.cfg.RetryBackoff
	eb.MaxInterval = 30 * time.Second

	attempt := 0
	operation := func() (struct{}, error) {
		attempt++
		lastErr := fn(ctx)
		if lastErr == nil {
			return struct{}{}, nil
		}

		if isRetryable(lastErr) {
			// Retryable errors (429, 5xx) should be retried with backoff.
			// They don't trip the circuit breaker.
			logx.WithContext(ctx).Infof("ai: retryable error on model %s (attempt %d): %v", modelID, attempt, lastErr)
			return struct{}{}, lastErr
		}

		// Non-retryable errors go through the circuit breaker.
		// If the breaker is open, stop retrying immediately.
		brk := c.breakerFor(modelID)
		if err := brk.DoWithAcceptable(func() error {
			return lastErr
		}, acceptable); err != nil {
			logx.WithContext(ctx).Infof("ai: circuit breaker open for model %s: %v", modelID, err)
			return struct{}{}, backoff.Permanent(fmt.Errorf("ai: circuit breaker: %w", err))
		}

		// Non-retryable, non-breaker error: don't retry.
		return struct{}{}, backoff.Permanent(lastErr)
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

// acceptable determines which errors the circuit breaker should count as failures.
func acceptable(err error) bool {
	if err == nil {
		return true
	}
	// Non-retryable errors (4xx except 429) should not trip the breaker.
	return !isRetryable(err)
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
