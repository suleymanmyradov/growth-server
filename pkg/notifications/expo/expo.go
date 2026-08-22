// Package expo implements a client for the Expo Push Notifications service
// (https://exp.host/--/api/v2/push/send). Expo abstracts APNs (iOS) and FCM
// (Android) behind a single HTTP API, so the backend only needs to know the
// Expo push token (ExponentPushToken[...]) returned by getExpoPushTokenAsync
// on the client.
//
// See docs/push-notifications-design.md for the full delivery flow:
//   - The mobile app calls POST /api/v1/devices/:installationId on startup
//     to register its Expo push token.
//   - When the notifications service wants to deliver a push, it looks up the
//     user's active devices and calls this client to send the push.
//   - Expo returns tickets immediately; receipts (delivery confirmations or
//     errors like DeviceNotRegistered) are fetched separately via CheckReceipts.
//     A DeviceNotRegistered receipt means the token is stale and the device
//     row should be disabled via DevicesRepo.DisableDeviceByToken.
package expo

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
	// sendURL is the Expo push send endpoint.
	sendURL = "https://exp.host/--/api/v2/push/send"
	// receiptURL is the Expo push receipts endpoint.
	receiptURL = "https://exp.host/--/api/v2/push/getReceipts"
	// DefaultTimeout for HTTP calls to Expo.
	DefaultTimeout = 10 * time.Second
)

// PushMessage is a single push notification payload sent to one Expo token.
// See https://docs.expo.dev/push-notifications/sending-notifications/.
type PushMessage struct {
	// To is the Expo push token (ExponentPushToken[...]).
	To string `json:"to"`
	// Title is the notification title.
	Title string `json:"title,omitempty"`
	// Body is the notification body text.
	Body string `json:"body,omitempty"`
	// Data is an arbitrary JSON object delivered to the app on tap. Use this
	// for deep-link routing (e.g. {"type":"habit_reminder","habitId":"..."}).
	Data map[string]any `json:"data,omitempty"`
	// Sound is the sound file name; 'default' plays the system sound.
	Sound string `json:"sound,omitempty"`
	// Badge sets the app icon badge count (iOS).
	Badge *int `json:"badge,omitempty"`
	// Priority 'default' | 'normal' | 'high'. 'high' for time-sensitive reminders.
	Priority string `json:"priority,omitempty"`
	// ChannelID (Android) maps to a notification channel created on the client.
	ChannelID string `json:"channelId,omitempty"`
	// MutableContent (iOS 10+) allows the notification service extension to
	// modify the payload (e.g. for notification extensions).
	MutableContent bool `json:"_mutableContent,omitempty"`
	// CategoryID (iOS) for actionable notifications (buttons).
	CategoryID string `json:"categoryIdentifier,omitempty"`
	// Expiration is the time after which Expo will not attempt to deliver.
	// Expressed as seconds since epoch per Expo API.
	Expiration *int64 `json:"expiration,omitempty"`
	// TTL is how long (seconds) Expo keeps the message if the device is offline.
	TTL *int64 `json:"ttl,omitempty"`
}

// Ticket is Expo's immediate response for one push send. A status of 'ok'
// means Expo accepted the message and will attempt delivery; the actual
// delivery result comes later via a receipt (CheckReceipts).
type Ticket struct {
	// Status is 'ok' or 'error'.
	Status string `json:"status"`
	// ID is the ticket ID, used to fetch the receipt later. Present on 'ok'.
	ID string `json:"id,omitempty"`
	// Message is the error message. Present on 'error'.
	Message string `json:"message,omitempty"`
	// Details carries error-specific details (e.g. {"error":"DeviceNotRegistered"}).
	Details map[string]string `json:"details,omitempty"`
}

// Receipt is Expo's delivery confirmation (or error) for a previously sent
// ticket. Fetched via CheckReceipts using the ticket ID.
type Receipt struct {
	// Status is 'ok' or 'error'.
	Status string `json:"status"`
	// Message is the error message. Present on 'error'.
	Message string `json:"message,omitempty"`
	// Details carries error-specific details. The key 'error' may be
	// 'DeviceNotRegistered' (token stale) or 'MessageRateExceeded'.
	Details map[string]string `json:"details,omitempty"`
}

// SendResponse is the top-level response from the send endpoint.
type SendResponse struct {
	// Data is the list of tickets, one per message in the request, in order.
	Data []Ticket `json:"data"`
}

// ReceiptsResponse is the top-level response from the receipts endpoint.
type ReceiptsResponse struct {
	// Data maps ticket ID → receipt.
	Data map[string]Receipt `json:"data"`
}

// Client sends push notifications via the Expo Push service. The HTTP client
// may be injected for tests.
type Client struct {
	httpClient  *http.Client
	sendURL     string
	receiptURL  string
	accessToken string // optional Bearer token for authenticated requests
}

// Option configures a Client. Used in tests to point the client at a mock
// server; in production the defaults are used.
type Option func(*Client)

// WithSendURL overrides the Expo push send endpoint. Intended for tests.
func WithSendURL(url string) Option { return func(c *Client) { c.sendURL = url } }

// WithReceiptURL overrides the Expo receipts endpoint. Intended for tests.
func WithReceiptURL(url string) Option { return func(c *Client) { c.receiptURL = url } }

// NewClient returns an Expo push client. If httpClient is nil, a client with
// DefaultTimeout is used. accessToken is optional (Expo allows unauthenticated
// sends; a token raises rate limits). Options may override the endpoints
// (intended for tests with a mock HTTP server).
func NewClient(httpClient *http.Client, accessToken string, opts ...Option) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}
	c := &Client{
		httpClient:  httpClient,
		sendURL:     sendURL,
		receiptURL:  receiptURL,
		accessToken: accessToken,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Send pushes up to 100 messages in a single request (Expo's per-request limit).
// Returns the tickets in the same order as the input messages.
func (c *Client) Send(ctx context.Context, messages []PushMessage) ([]Ticket, error) {
	if len(messages) == 0 {
		return nil, nil
	}
	if len(messages) > 100 {
		return nil, fmt.Errorf("expo: send accepts at most 100 messages per request (got %d)", len(messages))
	}
	for i, m := range messages {
		if !strings.HasPrefix(m.To, "ExponentPushToken[") && !strings.HasPrefix(m.To, "ExpoPushToken[") {
			return nil, fmt.Errorf("expo: message %d has invalid token %q (must start with ExponentPushToken[ or ExpoPushToken[)", i, m.To)
		}
	}

	body, err := json.Marshal(messages)
	if err != nil {
		return nil, fmt.Errorf("expo: marshal messages: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.sendURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("expo: send request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expo: send status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed SendResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("expo: decode send response: %w", err)
	}
	if len(parsed.Data) != len(messages) {
		return nil, fmt.Errorf("expo: ticket count mismatch (sent %d, got %d)", len(messages), len(parsed.Data))
	}
	return parsed.Data, nil
}

// SendOne is a convenience wrapper for a single message.
func (c *Client) SendOne(ctx context.Context, msg PushMessage) (Ticket, error) {
	tickets, err := c.Send(ctx, []PushMessage{msg})
	if err != nil {
		return Ticket{}, err
	}
	return tickets[0], nil
}

// CheckReceipts fetches delivery receipts for the given ticket IDs. Returns a
// map of ticketID → receipt. A receipt with status 'error' and details.error
// 'DeviceNotRegistered' means the token is stale and the device row should be
// disabled.
func (c *Client) CheckReceipts(ctx context.Context, ticketIDs []string) (map[string]Receipt, error) {
	if len(ticketIDs) == 0 {
		return map[string]Receipt{}, nil
	}
	if len(ticketIDs) > 1000 {
		return nil, fmt.Errorf("expo: receipts accepts at most 1000 ids per request (got %d)", len(ticketIDs))
	}

	body, err := json.Marshal(map[string][]string{"ids": ticketIDs})
	if err != nil {
		return nil, fmt.Errorf("expo: marshal receipt ids: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.receiptURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("expo: receipts request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expo: receipts status %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var parsed ReceiptsResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("expo: decode receipts response: %w", err)
	}
	return parsed.Data, nil
}

// setHeaders applies the headers Expo requires on every request.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Encoding", "gzip, deflate")
	if c.accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.accessToken)
	}
}

// IsDeviceNotRegistered returns true if a receipt's details indicate the token
// is no longer valid (the user uninstalled the app or revoked permission).
// Callers should disable the device row when this returns true.
func IsDeviceNotRegistered(r Receipt) bool {
	return r.Status == "error" && r.Details["error"] == "DeviceNotRegistered"
}
