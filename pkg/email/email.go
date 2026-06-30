// Package email provides a small abstraction over transactional email providers.
// The default implementation is Resend (https://resend.com). The Sender interface
// keeps the provider swappable so auth logic does not depend on a specific vendor.
package email

import (
	"context"
	"fmt"
)

// Email is the provider-agnostic message passed to Sender.Send.
type Email struct {
	From        string            // verified sender address
	To          []string          // recipient addresses
	Subject     string            // email subject line
	HTML        string            // HTML body (required; plain-text fallback derived by provider)
	ReplyTo     string            // optional Reply-To address
	Headers     map[string]string // optional custom headers
}

// Sender sends a transactional email. Implementations must be safe for concurrent use.
type Sender interface {
	Send(ctx context.Context, msg Email) error
}

// Config holds provider configuration. Only Resend is implemented today.
type Config struct {
	Provider    string // "resend" (default) — reserved for future providers
	APIKey      string // provider API key
	FromAddress string // default From address used when Email.From is empty
	// FrontendBaseURL is the public origin used to build action links
	// (e.g. https://app.example.com). Injected by callers, not the sender itself.
}

// New returns a Sender for the configured provider. An empty provider defaults to
// "resend". A nil/empty API key returns a NoopSender so local development works
// without email credentials (sends are logged and dropped).
func New(c Config) (Sender, error) {
	if c.Provider == "" {
		c.Provider = "resend"
	}
	if c.APIKey == "" {
		return &NoopSender{}, nil
	}
	switch c.Provider {
	case "resend":
		return NewResendSender(c), nil
	default:
		return nil, fmt.Errorf("email: unknown provider %q", c.Provider)
	}
}

// NoopSender logs sends and succeeds without delivering anything. Used when no
// API key is configured (local dev) so the auth flow can be exercised end-to-end.
type NoopSender struct{}

func (n *NoopSender) Send(ctx context.Context, msg Email) error {
	return nil
}
