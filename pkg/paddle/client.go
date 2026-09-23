package paddle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// Paddle API base URLs. Paddle Billing runs two fully separate environments —
// credentials, catalogs, and webhook secrets never cross over.
const (
	LiveBaseURL    = "https://api.paddle.com"
	SandboxBaseURL = "https://sandbox-api.paddle.com"
)

// defaultHTTPTimeout bounds every Paddle API call. Webhook processing must
// ack within seconds, so keep this tight.
const defaultHTTPTimeout = 30 * time.Second

// paddleAPIVersion is the Paddle-Version header Paddle requires on every
// request. "1" is the only published API version today.
const paddleAPIVersion = "1"

// Client is a thin Paddle Billing API client. Construct with NewClient.
type Client struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

// ClientOption configures a Client.
type ClientOption func(*Client)

// WithBaseURL overrides the API base URL — used in tests to point at a
// local httptest server.
func WithBaseURL(u string) ClientOption {
	return func(c *Client) { c.baseURL = u }
}

// NewClient returns a Paddle API client. sandbox=true targets the sandbox
// environment (pdl_sdbx_ keys); false targets live (pdl_live_ keys). A nil
// httpClient uses a default with a bounded timeout.
func NewClient(apiKey string, sandbox bool, httpClient *http.Client, opts ...ClientOption) *Client {
	base := LiveBaseURL
	if sandbox {
		base = SandboxBaseURL
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	c := &Client{apiKey: apiKey, baseURL: base, httpClient: httpClient}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// APIError is a non-2xx response from the Paddle API.
type APIError struct {
	// StatusCode is the HTTP status.
	StatusCode int
	// Code is Paddle's machine-readable error code (e.g. "bad_request").
	Code string
	// Detail is the human-readable message.
	Detail string
	// RequestID is Paddle's request id — quote it in support tickets.
	RequestID string
	// RawBody holds the response body when it wasn't a Paddle error object
	// (e.g. an HTML error page from a proxy).
	RawBody string
}

func (e *APIError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("paddle: %s (%s, http %d, request %s)", e.Detail, e.Code, e.StatusCode, e.RequestID)
	}
	return fmt.Sprintf("paddle: http %d (request %s)", e.StatusCode, e.RequestID)
}

// apiEnvelope wraps every Paddle API response.
type apiEnvelope struct {
	Data json.RawMessage `json:"data"`
	Meta struct {
		RequestID string `json:"request_id"`
	} `json:"meta"`
	Error *struct {
		Type   string `json:"type"`
		Code   string `json:"code"`
		Detail string `json:"detail"`
	} `json:"error"`
}

// do performs one API call and returns the decoded data payload.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var bodyReader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("paddle: encode request: %w", err)
		}
		bodyReader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, u, bodyReader)
	if err != nil {
		return fmt.Errorf("paddle: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Paddle-Version", paddleAPIVersion)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("paddle: %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("paddle: read response: %w", err)
	}

	var env apiEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		if resp.StatusCode >= 400 {
			return &APIError{StatusCode: resp.StatusCode, RawBody: string(raw)}
		}
		return fmt.Errorf("paddle: decode response: %w", err)
	}

	if resp.StatusCode >= 400 || env.Error != nil {
		apiErr := &APIError{StatusCode: resp.StatusCode, RequestID: env.Meta.RequestID}
		if env.Error != nil {
			apiErr.Code = env.Error.Code
			apiErr.Detail = env.Error.Detail
		} else {
			apiErr.RawBody = string(raw)
		}
		return apiErr
	}

	if out != nil {
		if err := json.Unmarshal(env.Data, out); err != nil {
			return fmt.Errorf("paddle: decode data: %w", err)
		}
	}
	return nil
}

// Customer is a Paddle customer entity (fields we consume only).
type Customer struct {
	ID     string `json:"id"`
	Email  string `json:"email"`
	Name   string `json:"name"`
	Status string `json:"status"`
}

// CreateCustomer creates a customer. Paddle dedupes on email.
func (c *Client) CreateCustomer(ctx context.Context, email, name string, customData map[string]any) (*Customer, error) {
	if email == "" {
		return nil, errors.New("paddle: email is required")
	}
	body := map[string]any{"email": email}
	if name != "" {
		body["name"] = name
	}
	if len(customData) > 0 {
		body["custom_data"] = customData
	}
	var cust Customer
	if err := c.do(ctx, http.MethodPost, "/customers", nil, body, &cust); err != nil {
		return nil, err
	}
	return &cust, nil
}

// TransactionItemParam is a line item on a new transaction.
type TransactionItemParam struct {
	// PriceID is the catalog price (pri_...).
	PriceID string
	// Quantity defaults to 1 when zero.
	Quantity int
}

// CreateTransactionParams are the inputs to CreateTransaction.
type CreateTransactionParams struct {
	// Items is the list of prices to charge. At least one required.
	Items []TransactionItemParam
	// CustomerID references an existing Paddle customer (ctm_...). When
	// empty, CustomerEmail is used to create one first.
	CustomerID string
	// CustomerEmail / CustomerName are used to create a customer when
	// CustomerID is empty.
	CustomerEmail string
	CustomerName  string
	// UserID is the internal user ID — stamped into custom_data so webhooks
	// can map the payment back to a user without a lookup.
	UserID string
	// CustomData is merged into custom_data alongside user_id.
	CustomData map[string]any
	// CheckoutURL is the hosted payment page URL (Paddle.js target). When
	// empty, Paddle falls back to the account's default payment link.
	CheckoutURL string
	// SuccessURL / CancelURL ride to the Paddle.js page as query params on
	// the checkout URL — Paddle Billing transactions have no success_url
	// field of their own.
	SuccessURL string
	CancelURL  string
}

// CreateTransaction creates a checkout transaction and returns it; the
// payment link is txn.CheckoutURL(). Requires automatic collection (hosted
// checkout) — manual invoicing is out of scope for web self-serve checkout.
func (c *Client) CreateTransaction(ctx context.Context, p CreateTransactionParams) (*Transaction, error) {
	if c.apiKey == "" {
		return nil, errors.New("paddle: API key is not configured")
	}
	if len(p.Items) == 0 {
		return nil, errors.New("paddle: at least one item is required")
	}
	for i, item := range p.Items {
		if item.PriceID == "" {
			return nil, fmt.Errorf("paddle: items[%d].price_id is required", i)
		}
	}

	// Customer is optional — hosted checkout collects email when absent.
	// When only an email is known, create the customer up front so events
	// can correlate on a stable ctm_ id.
	customerID := p.CustomerID
	if customerID == "" && p.CustomerEmail != "" {
		cust, err := c.CreateCustomer(ctx, p.CustomerEmail, p.CustomerName, nil)
		if err != nil {
			return nil, fmt.Errorf("paddle: create customer: %w", err)
		}
		customerID = cust.ID
	}

	customData := map[string]any{}
	for k, v := range p.CustomData {
		customData[k] = v
	}
	if p.UserID != "" {
		customData["user_id"] = p.UserID
	}

	items := make([]map[string]any, 0, len(p.Items))
	for _, item := range p.Items {
		qty := item.Quantity
		if qty <= 0 {
			qty = 1
		}
		items = append(items, map[string]any{"price_id": item.PriceID, "quantity": qty})
	}

	body := map[string]any{
		"collection_mode": "automatic",
		"items":           items,
	}
	if customerID != "" {
		body["customer_id"] = customerID
	}
	if len(customData) > 0 {
		body["custom_data"] = customData
	}
	checkoutURL, err := checkoutURLWithRedirects(p.CheckoutURL, p.SuccessURL, p.CancelURL)
	if err != nil {
		return nil, fmt.Errorf("paddle: checkout url: %w", err)
	}
	if checkoutURL != "" {
		body["checkout"] = map[string]any{"url": checkoutURL}
	}

	var txn Transaction
	if err := c.do(ctx, http.MethodPost, "/transactions", nil, body, &txn); err != nil {
		return nil, err
	}
	return &txn, nil
}

// checkoutURLWithRedirects appends success_url/cancel_url query params to the
// checkout URL. Empty checkout URL returns "" — Paddle then uses the default
// payment link and there's nothing to hang params on.
func checkoutURLWithRedirects(checkoutURL, successURL, cancelURL string) (string, error) {
	if checkoutURL == "" {
		return "", nil
	}
	u, err := url.Parse(checkoutURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	if successURL != "" {
		q.Set("success_url", successURL)
	}
	if cancelURL != "" {
		q.Set("cancel_url", cancelURL)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

// GetSubscription fetches one subscription by id (sub_...).
func (c *Client) GetSubscription(ctx context.Context, subscriptionID string) (*Subscription, error) {
	if subscriptionID == "" {
		return nil, errors.New("paddle: subscription ID is required")
	}
	var sub Subscription
	if err := c.do(ctx, http.MethodGet, "/subscriptions/"+subscriptionID, nil, nil, &sub); err != nil {
		return nil, err
	}
	return &sub, nil
}

// ListSubscriptions fetches a customer's subscriptions. Only the first page
// is returned — enough for backfill/reconciliation of a single-user product.
func (c *Client) ListSubscriptions(ctx context.Context, customerID string) ([]Subscription, error) {
	if customerID == "" {
		return nil, errors.New("paddle: customer ID is required")
	}
	q := url.Values{"customer_id": {customerID}}
	var subs []Subscription
	if err := c.do(ctx, http.MethodGet, "/subscriptions", q, nil, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}

// CancelSubscription schedules cancellation at the end of the current billing
// period (effective_from=next_billing_period) — the user keeps access until
// the paid-through date, matching the subscriptions.cancel_at_period_end flag.
func (c *Client) CancelSubscription(ctx context.Context, subscriptionID string) (*Subscription, error) {
	if subscriptionID == "" {
		return nil, errors.New("paddle: subscription ID is required")
	}
	var sub Subscription
	body := map[string]any{"effective_from": "next_billing_period"}
	if err := c.do(ctx, http.MethodPost, "/subscriptions/"+subscriptionID+"/cancel", nil, body, &sub); err != nil {
		return nil, err
	}
	return &sub, nil
}
