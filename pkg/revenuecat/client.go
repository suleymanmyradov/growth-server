// Package revenuecat implements a client for the RevenueCat REST API
// (https://www.revenuecat.com/api/v2). RevenueCat abstracts App Store, Play
// Store, and Amazon Store subscriptions behind a single API, so the backend
// can manage mobile subscriptions without dealing with store-specific APIs.
//
// The primary use cases for this client are:
//   - Verifying webhook signatures (the webhook secret is configured in the
//     RevenueCat dashboard).
//   - Fetching customer entitlements (GET /v2/projects/{project_id}/customers/{customer_id}/entitlements)
//     for reconciliation and backfill.
//   - Backfilling existing Stripe subscribers into RevenueCat (POST
//     /v2/projects/{project_id}/customers/{customer_id}/subscriptions) so
//     RevenueCat has a complete picture of the user's purchases.
//
// See docs/push-notifications-design.md (billing section) for the full flow.
package revenuecat

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
	// baseURL is the RevenueCat API v2 base URL.
	baseURL = "https://api.revenuecat.com/v2"
	// DefaultTimeout for HTTP calls to RevenueCat.
	DefaultTimeout = 15 * time.Second
)

// Client calls the RevenueCat REST API. The HTTP client may be injected for
// tests.
type Client struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	projectID  string
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the RevenueCat API base URL. Intended for tests.
func WithBaseURL(url string) Option { return func(c *Client) { c.baseURL = url } }

// NewClient returns a RevenueCat API client. apiKey is the secret API key from
// the RevenueCat dashboard (starts with "sk_"). projectID is the RevenueCat
// project ID. httpClient may be nil (a client with DefaultTimeout is used).
func NewClient(apiKey, projectID string, httpClient *http.Client, opts ...Option) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: DefaultTimeout}
	}
	c := &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
		apiKey:     apiKey,
		projectID:  projectID,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Entitlement represents a RevenueCat entitlement (e.g., "pro").
type Entitlement struct {
	ID             string     `json:"id"`
	ProductID      string     `json:"product_id"`
	Store          string     `json:"store"`
	PurchaseDate   time.Time  `json:"purchase_date"`
	ExpirationDate *time.Time `json:"expiration_date,omitempty"`
	IsActive       bool       `json:"is_active"`
}

// EntitlementsResponse is the response from GET .../customers/{id}/entitlements.
type EntitlementsResponse struct {
	Items []Entitlement `json:"items"`
}

// GetCustomerEntitlements fetches the active entitlements for a RevenueCat
// customer. customerID is the app_user_id passed to Purchases.logIn() on the
// mobile client (usually our user UUID).
func (c *Client) GetCustomerEntitlements(ctx context.Context, customerID string) ([]Entitlement, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("revenuecat: API key is not configured")
	}
	if c.projectID == "" {
		return nil, fmt.Errorf("revenuecat: project ID is not configured")
	}
	if customerID == "" {
		return nil, fmt.Errorf("revenuecat: customer ID is required")
	}

	url := fmt.Sprintf("%s/projects/%s/customers/%s/entitlements",
		c.baseURL, c.projectID, customerID)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("revenuecat: get entitlements: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("revenuecat: get entitlements status %d: %s",
			resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var parsed EntitlementsResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("revenuecat: decode entitlements: %w", err)
	}
	return parsed.Items, nil
}

// HasProEntitlement returns true if the customer has an active "pro" entitlement.
func HasProEntitlement(entitlements []Entitlement) bool {
	for _, e := range entitlements {
		if e.ID == "pro" && e.IsActive {
			return true
		}
	}
	return false
}

// WebhookEvent is a single event in a RevenueCat webhook payload. RevenueCat
// sends an array of events in the webhook body. See:
// https://docs.revenuecat.com/integrations/webhooks/event-types-and-fields
type WebhookEvent struct {
	// Type is the event type: NON_RENEWING_PURCHASE, INITIAL_PURCHASE,
	// RENEWAL, CANCELLATION, UNCANCELLATION, EXPIRATION, PRODUCT_CHANGE,
	// ENTITLEMENT_CHANGE, TRANSFER.
	Type string `json:"type"`
	// Store is the store: APP_STORE, PLAY_STORE, AMAZON, STRIPE, PROMO.
	Store string `json:"store"`
	// AppUserID is the user ID passed to Purchases.logIn() (our user UUID).
	AppUserID string `json:"app_user_id"`
	// OriginalAppUserID is the original anonymous user ID (before logIn).
	OriginalAppUserID string `json:"original_app_user_id"`
	// ProductID is the store product ID.
	ProductID string `json:"product_id"`
	// EntitlementID is the entitlement ID affected (e.g., "pro"). May be empty.
	EntitlementID string `json:"entitlement_id"`
	// PeriodStartAt is the start of the subscription period (ISO 8601).
	PeriodStartAt string `json:"period_start_at"`
	// ExpirationAt is the end of the subscription period (ISO 8601). May be
	// empty for non-renewing purchases.
	ExpirationAt string `json:"expiration_at"`
	// EventID is a unique identifier for the event (used for idempotency).
	// RevenueCat does not always send this; we derive one if absent.
	EventID string `json:"event_id"`
}

// WebhookPayload is the top-level webhook body. RevenueCat sends:
// {"events": [...]}
type WebhookPayload struct {
	Events []WebhookEvent `json:"events"`
}

// ParseWebhookPayload parses the webhook body. It does NOT verify the
// signature — that is done separately via VerifyWebhookSignature so the
// handler can reject unauthenticated requests before parsing.
func ParseWebhookPayload(body []byte) (WebhookPayload, error) {
	var payload WebhookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		return payload, fmt.Errorf("revenuecat: parse webhook payload: %w", err)
	}
	return payload, nil
}

// VerifyWebhookSignature verifies the Authorization header against the
// configured webhook secret. RevenueCat sends the secret as a Bearer token:
//
//	Authorization: Bearer <webhook_secret>
//
// The comparison is constant-time to prevent timing attacks.
func VerifyWebhookSignature(authHeader, webhookSecret string) bool {
	if webhookSecret == "" {
		return false
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(authHeader, prefix) {
		return false
	}
	token := strings.TrimSpace(authHeader[len(prefix):])
	return subtleEqual(token, webhookSecret)
}

// subtleEqual compares two strings in constant time.
func subtleEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

// setHeaders applies the headers RevenueCat requires on every API request.
func (c *Client) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
}

// PostSubscription is used for Stripe backfill: notifies RevenueCat of an
// existing Stripe subscription so RevenueCat has a complete purchase history.
// This is a no-op if the user already has the entitlement in RevenueCat.
type PostSubscriptionRequest struct {
	ProductID    string `json:"product_id"`
	Store        string `json:"store"`
	Price        int64  `json:"price"`
	Currency     string `json:"currency"`
	PurchaseDate string `json:"purchase_date"`
}

// PostSubscription records a purchase in RevenueCat for backfill purposes.
// This is used when a user has an existing Stripe subscription and we want
// RevenueCat to know about it (e.g., for cross-platform entitlement tracking).
func (c *Client) PostSubscription(ctx context.Context, customerID string, req PostSubscriptionRequest) error {
	if c.apiKey == "" || c.projectID == "" {
		return fmt.Errorf("revenuecat: API key and project ID are required")
	}
	if customerID == "" {
		return fmt.Errorf("revenuecat: customer ID is required")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("revenuecat: marshal subscription: %w", err)
	}

	url := fmt.Sprintf("%s/projects/%s/customers/%s/purchases",
		c.baseURL, c.projectID, customerID)

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("revenuecat: post subscription: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("revenuecat: post subscription status %d: %s",
			resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	return nil
}
