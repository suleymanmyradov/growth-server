package email

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	resendAPIURL   = "https://api.resend.com/emails"
	resendTimeout  = 10 * time.Second
)

// ResendSender sends email via the Resend HTTP API.
type ResendSender struct {
	apiKey      string
	fromAddress string
	httpClient  *http.Client
}

// NewResendSender builds a Resend-backed Sender.
func NewResendSender(c Config) *ResendSender {
	return &ResendSender{
		apiKey:      c.APIKey,
		fromAddress: c.FromAddress,
		httpClient:  &http.Client{Timeout: resendTimeout},
	}
}

type resendSendRequest struct {
	From    string            `json:"from"`
	To      []string          `json:"to"`
	Subject string            `json:"subject"`
	HTML    string            `json:"html"`
	ReplyTo string            `json:"reply_to,omitempty"`
	Headers map[string]string `json:"headers,omitempty"`
}

type resendErrorResponse struct {
	StatusCode int    `json:"statusCode"`
	Message    string `json:"message"`
	Name       string `json:"name"`
}

func (r *ResendSender) Send(ctx context.Context, msg Email) error {
	from := msg.From
	if strings.TrimSpace(from) == "" {
		from = r.fromAddress
	}
	if from == "" {
		return fmt.Errorf("email: from address is required")
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("email: at least one recipient is required")
	}

	body := resendSendRequest{
		From:    from,
		To:      msg.To,
		Subject: msg.Subject,
		HTML:    msg.HTML,
		ReplyTo: msg.ReplyTo,
		Headers: msg.Headers,
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("email: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, resendAPIURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("email: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+r.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("email: resend request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	raw, _ := io.ReadAll(resp.Body)
	var apiErr resendErrorResponse
	if json.Unmarshal(raw, &apiErr) == nil && apiErr.Message != "" {
		return fmt.Errorf("email: resend API error %d: %s", resp.StatusCode, apiErr.Message)
	}
	return fmt.Errorf("email: resend API error %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
}
