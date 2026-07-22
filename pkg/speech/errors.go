package speech

import "errors"

var (
	// ErrNotConfigured is returned when a speech client is used but no provider
	// is configured (e.g. STT model is empty).
	ErrNotConfigured = errors.New("speech: not configured")
	// ErrUnsupportedFormat is returned when the audio format is not supported
	// by the selected provider.
	ErrUnsupportedFormat = errors.New("speech: unsupported audio format")
	// ErrUnsupportedProvider is returned when the configured provider string
	// does not match any known implementation.
	ErrUnsupportedProvider = errors.New("speech: unsupported provider")
)

// ProviderError wraps a non-2xx response from a speech provider.
type ProviderError struct {
	Provider string
	Status   int
	Body     string
}

func (e *ProviderError) Error() string {
	return "speech: provider " + e.Provider + " returned status " + itoa(e.Status) + ": " + e.Body
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
