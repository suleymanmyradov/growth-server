package ai

import (
	"errors"
	"fmt"
)

// Sentinel errors for the ai package.
var (
	// ErrQuotaExceeded is returned when a user or global daily quota is exceeded.
	ErrQuotaExceeded = errors.New("ai: quota exceeded")

	// ErrSafetyBlock is returned when the safety classifier blocks input.
	ErrSafetyBlock = errors.New("ai: safety block")

	// ErrModelUnavailable is returned when the model cannot be reached after retries.
	ErrModelUnavailable = errors.New("ai: model unavailable")

	// ErrInvalidProfile is returned when an unknown ModelProfile is used.
	ErrInvalidProfile = errors.New("ai: invalid model profile")

	// ErrNoTools is returned when RunAgent is called with no tools.
	ErrNoTools = errors.New("ai: agent requires at least one tool")

	// ErrMaxSteps is returned when the agent loop exceeds MaxSteps.
	ErrMaxSteps = errors.New("ai: agent exceeded max steps")

	// ErrStreamClosed is returned when reading from a closed stream.
	ErrStreamClosed = errors.New("ai: stream closed")

	// ErrConfigInvalid is returned when config validation fails.
	ErrConfigInvalid = errors.New("ai: config invalid")
)

// QuotaError provides details about which quota was exceeded.
type QuotaError struct {
	Limit  string
	Used   int64
	Cap   int64
}

func (e *QuotaError) Error() string {
	return fmt.Sprintf("ai: %s quota exceeded (used %d, cap %d)", e.Limit, e.Used, e.Cap)
}

func (e *QuotaError) Unwrap() error {
	return ErrQuotaExceeded
}

// SafetyError provides details about why input was blocked.
type SafetyError struct {
	Category   string
	Confidence float64
	Reason     string
}

func (e *SafetyError) Error() string {
	return fmt.Sprintf("ai: safety block (%s, confidence %.2f): %s", e.Category, e.Confidence, e.Reason)
}

func (e *SafetyError) Unwrap() error {
	return ErrSafetyBlock
}
