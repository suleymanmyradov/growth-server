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

	// ErrMaxTokens is returned when the agent loop exceeds the cumulative
	// MaxTotalTokens budget. This is distinct from ErrMaxSteps so callers
	// can differentiate between "too many round-trips" and "too many
	// tokens" — the former suggests the model is stuck in a tool-call
	// loop, the latter suggests the conversation is too long or the
	// budget is too small.
	ErrMaxTokens = errors.New("ai: agent exceeded max total tokens")

	// ErrStreamClosed is returned when reading from a closed stream.
	ErrStreamClosed = errors.New("ai: stream closed")

	// ErrStreamIncomplete is returned when a provider ends a stream without
	// confirming a normal completion.
	ErrStreamIncomplete = errors.New("ai: stream incomplete")

	// ErrConfigInvalid is returned when config validation fails.
	ErrConfigInvalid = errors.New("ai: config invalid")
)

// QuotaError provides details about which quota was exceeded.
type QuotaError struct {
	Limit string
	Used  int64
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

// UserFacingMessage converts an ai package error into a short, user-friendly
// message suitable for display in chat/UI. Internal implementation details
// (token counts, step limits, library names, gRPC codes) never leak to end
// users — known sentinel errors get a tailored message, everything else gets
// a generic fallback.
func UserFacingMessage(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case errors.Is(err, ErrMaxSteps):
		return "I'm having trouble putting my thoughts together on this one. Could you try rephrasing your message or asking something more specific?"
	case errors.Is(err, ErrMaxTokens):
		return "This conversation is getting quite long. Could you try starting a new conversation or asking something more specific?"
	case errors.Is(err, ErrQuotaExceeded):
		return "You've reached your daily coaching limit. I'll be here to help again tomorrow — see you then!"
	case errors.Is(err, ErrSafetyBlock):
		return "I'm not able to help with that, but I'm here if you'd like to talk about your goals or habits."
	case errors.Is(err, ErrModelUnavailable):
		return "I'm having trouble connecting right now. Please try again in a moment."
	case errors.Is(err, ErrStreamIncomplete):
		return "I lost my response before I could finish. Please try sending your message again."
	default:
		return "Something went wrong on my end. Please try sending your message again."
	}
}
