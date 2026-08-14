package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"time"

	"github.com/cenkalti/backoff/v5"
	openaimodel "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/zeromicro/go-zero/core/logx"
)

// retryFn is the function signature for operations that can be retried.
type retryFn func(ctx context.Context) error

// retryDelayBackoff wraps an ExponentialBackOff, allowing a per-error override
// for the next backoff interval. When a provider returns a 429 with an explicit
// retry delay (e.g. Google AI Studio's "Please retry in 35s"), the operation
// sets the override so the next wait matches the provider's recommendation
// instead of the exponential schedule.
type retryDelayBackoff struct {
	eb       *backoff.ExponentialBackOff
	override time.Duration // if > 0, returned once then cleared
}

func (b *retryDelayBackoff) NextBackOff() time.Duration {
	if b.override > 0 {
		d := b.override
		b.override = 0
		// Still advance the exponential backoff's internal state so the
		// subsequent interval (if the override is consumed and the error
		// persists) continues from where it would have been.
		_ = b.eb.NextBackOff()
		return d
	}
	return b.eb.NextBackOff()
}

func (b *retryDelayBackoff) Reset() { b.eb.Reset() }

// retryDelayRegex matches "Please retry in 35.68s" (Google AI Studio) and
// similar patterns like "retry in 30s" or "retry_delay": "42s".
var retryDelayRegex = regexp.MustCompile(`retry[^0-9]*(\d+(?:\.\d+)?)\s*s`)

// parseRetryDelay extracts a retry delay from an error message if the provider
// included one (e.g. Google AI Studio's 429 response body contains
// "Please retry in 35.683551572s"). Returns 0 if no delay is found.
func parseRetryDelay(err error) time.Duration {
	if err == nil {
		return 0
	}
	// Check Eino OpenAI adapter's APIError — its Message field carries the
	// provider's error message verbatim.
	var einoAPIErr *openaimodel.APIError
	if errors.As(err, &einoAPIErr) {
		if d := parseRetryDelayFromString(einoAPIErr.Message); d > 0 {
			return d
		}
	}
	// Check our internal apiError type.
	if apiErr, ok := unwrapAPIError(err); ok {
		if d := parseRetryDelayFromString(apiErr.Message); d > 0 {
			return d
		}
	}
	// Last resort: scan the full error string.
	return parseRetryDelayFromString(err.Error())
}

func parseRetryDelayFromString(s string) time.Duration {
	m := retryDelayRegex.FindStringSubmatch(s)
	if m == nil {
		return 0
	}
	secs, err := strconv.ParseFloat(m[1], 64)
	if err != nil || secs <= 0 {
		return 0
	}
	d := time.Duration(secs * float64(time.Second))
	// Cap at 60s to avoid excessively long waits that would exceed typical
	// context deadlines.
	if d > 60*time.Second {
		d = 60 * time.Second
	}
	return d
}

// withRetry wraps an operation with exponential backoff, per-attempt timeouts,
// and circuit breaker. Retryable errors (429/5xx) are retried without tripping
// the breaker; non-retryable errors count as breaker failures.
// When a provider includes an explicit retry delay in a 429 response (e.g.
// Google AI Studio's "Please retry in 35s"), that delay is honored instead of
// the exponential backoff interval for the next attempt.
func (c *client) withRetry(ctx context.Context, modelID string, fn retryFn) error {
	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = c.cfg.RetryBackoff
	eb.MaxInterval = 30 * time.Second
	rdb := &retryDelayBackoff{eb: eb}

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
			// If the provider included an explicit retry delay (e.g. Google
			// AI Studio's 429 "Please retry in 35s"), honor it for the next
			// backoff instead of the exponential interval.
			if delay := parseRetryDelay(lastErr); delay > 0 {
				rdb.override = delay
				logx.WithContext(ctx).Infof("ai: retryable error on model %s (attempt %d), provider retry delay %v: %v", modelID, attempt, delay, lastErr)
			} else {
				logx.WithContext(ctx).Infof("ai: retryable error on model %s (attempt %d): %v", modelID, attempt, lastErr)
			}
			return struct{}{}, lastErr
		}

		return struct{}{}, nil
	}

	_, err := backoff.Retry(ctx, operation,
		backoff.WithBackOff(rdb),
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
