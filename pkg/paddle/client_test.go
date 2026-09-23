package paddle

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPaddle spins up a test Paddle API and returns a client pointed at it.
func mockPaddle(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient("pdl_test_key", false, nil, WithBaseURL(srv.URL))
}

// readJSON reads and decodes the request body for assertions.
func readJSON(t *testing.T, r *http.Request, out any) {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(body, out))
}

// assertPaddleRequest checks the headers every Paddle API call must carry.
func assertPaddleRequest(t *testing.T, r *http.Request) {
	t.Helper()
	assert.Equal(t, "Bearer pdl_test_key", r.Header.Get("Authorization"))
	assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
	assert.Equal(t, "application/json", r.Header.Get("Accept"))
	assert.Equal(t, "1", r.Header.Get("Paddle-Version"))
}

func TestCreateTransaction_RequestShape(t *testing.T) {
	c := mockPaddle(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/transactions", r.URL.Path)
		assertPaddleRequest(t, r)

		var sent map[string]any
		readJSON(t, r, &sent)

		assert.Equal(t, "automatic", sent["collection_mode"])
		assert.Equal(t, "ctm_01h8441jn5pcwrfhwh78jqt8hk", sent["customer_id"])

		items, ok := sent["items"].([]any)
		require.True(t, ok)
		require.Len(t, items, 1)
		item := items[0].(map[string]any)
		assert.Equal(t, "pri_01gsz8x8sawmvhz1pv30nge1ke", item["price_id"])
		assert.Equal(t, float64(1), item["quantity"])

		customData := sent["custom_data"].(map[string]any)
		assert.Equal(t, "user-123", customData["user_id"])
		assert.Equal(t, "web", customData["source"])

		checkout := sent["checkout"].(map[string]any)
		checkoutURL, err := url.Parse(checkout["url"].(string))
		require.NoError(t, err)
		assert.Equal(t, "example.com", checkoutURL.Host)
		assert.Equal(t, "/checkout", checkoutURL.Path)
		// Success/cancel URLs ride to the Paddle.js page as query params.
		assert.Equal(t, "https://example.com/pricing?done=1", checkoutURL.Query().Get("success_url"))
		assert.Equal(t, "https://example.com/pricing?canceled=1", checkoutURL.Query().Get("cancel_url"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "txn_01hgk505qd8mm8yt0yvk1b6w8x",
				"status": "draft",
				"customer_id": "ctm_01h8441jn5pcwrfhwh78jqt8hk",
				"custom_data": {"user_id": "user-123", "source": "web"},
				"checkout": {"url": "https://example.com/checkout?_ptxn=txn_01hgk505qd8mm8yt0yvk1b6w8x"},
				"items": [{"price_id": "pri_01gsz8x8sawmvhz1pv30nge1ke", "quantity": 1}],
				"created_at": "2024-04-11T15:56:00.000Z",
				"updated_at": "2024-04-11T15:56:00.000Z"
			},
			"meta": {"request_id": "req_01"}
		}`))
	})

	txn, err := c.CreateTransaction(context.Background(), CreateTransactionParams{
		Items: []TransactionItemParam{
			{PriceID: "pri_01gsz8x8sawmvhz1pv30nge1ke", Quantity: 1},
		},
		CustomerID:  "ctm_01h8441jn5pcwrfhwh78jqt8hk",
		UserID:      "user-123",
		CustomData:  map[string]any{"source": "web"},
		CheckoutURL: "https://example.com/checkout",
		SuccessURL:  "https://example.com/pricing?done=1",
		CancelURL:   "https://example.com/pricing?canceled=1",
	})
	require.NoError(t, err)
	assert.Equal(t, "txn_01hgk505qd8mm8yt0yvk1b6w8x", txn.ID)
	assert.Equal(t, "user-123", txn.UserID())
	assert.Equal(t, "https://example.com/checkout?_ptxn=txn_01hgk505qd8mm8yt0yvk1b6w8x", txn.CheckoutURL())
}

func TestCreateTransaction_CustomerEmailFlow(t *testing.T) {
	// With only an email (no ctm_ id) the client creates the customer first.
	var calls []string
	c := mockPaddle(t, func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/customers":
			var sent map[string]any
			readJSON(t, r, &sent)
			assert.Equal(t, "jane@example.com", sent["email"])
			assert.Equal(t, "Jane", sent["name"])
			_, _ = w.Write([]byte(`{"data": {"id": "ctm_newcustomer01", "email": "jane@example.com", "name": "Jane", "status": "active"}, "meta": {"request_id": "req_c1"}}`))
		case "/transactions":
			var sent map[string]any
			readJSON(t, r, &sent)
			assert.Equal(t, "ctm_newcustomer01", sent["customer_id"])
			_, _ = w.Write([]byte(`{"data": {"id": "txn_01", "status": "draft", "customer_id": "ctm_newcustomer01", "checkout": {"url": "https://example.com/checkout?_ptxn=txn_01"}, "created_at": "2024-04-11T15:56:00.000Z", "updated_at": "2024-04-11T15:56:00.000Z"}, "meta": {"request_id": "req_t1"}}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})

	txn, err := c.CreateTransaction(context.Background(), CreateTransactionParams{
		Items:         []TransactionItemParam{{PriceID: "pri_01gsz8x8sawmvhz1pv30nge1ke"}},
		CustomerEmail: "jane@example.com",
		CustomerName:  "Jane",
		UserID:        "user-123",
	})
	require.NoError(t, err)
	assert.Equal(t, "txn_01", txn.ID)
	assert.Equal(t, []string{"POST /customers", "POST /transactions"}, calls)
}

func TestCreateTransaction_Validation(t *testing.T) {
	c := NewClient("pdl_test_key", false, nil)

	_, err := c.CreateTransaction(context.Background(), CreateTransactionParams{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "item")

	_, err = c.CreateTransaction(context.Background(), CreateTransactionParams{
		Items: []TransactionItemParam{{PriceID: ""}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "price_id")
}

func TestCreateTransaction_MissingAPIKey(t *testing.T) {
	c := NewClient("", false, nil)
	_, err := c.CreateTransaction(context.Background(), CreateTransactionParams{
		Items: []TransactionItemParam{{PriceID: "pri_x"}},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key")
}

func TestCreateTransaction_APIError(t *testing.T) {
	c := mockPaddle(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"error": {
				"type": "request_error",
				"code": "bad_request",
				"detail": "items.0.price_id is not a valid price"
			},
			"meta": {"request_id": "req_err_01"}
		}`))
	})

	_, err := c.CreateTransaction(context.Background(), CreateTransactionParams{
		Items: []TransactionItemParam{{PriceID: "pri_bad"}},
	})
	require.Error(t, err)

	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusBadRequest, apiErr.StatusCode)
	assert.Equal(t, "bad_request", apiErr.Code)
	assert.Equal(t, "items.0.price_id is not a valid price", apiErr.Detail)
	assert.Equal(t, "req_err_01", apiErr.RequestID)
}

func TestCreateTransaction_NonJSONError(t *testing.T) {
	c := mockPaddle(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte(`<html>bad gateway</html>`))
	})

	_, err := c.CreateTransaction(context.Background(), CreateTransactionParams{
		Items: []TransactionItemParam{{PriceID: "pri_x"}},
	})
	require.Error(t, err)

	var apiErr *APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusBadGateway, apiErr.StatusCode)
	assert.Contains(t, apiErr.RawBody, "bad gateway")
}

func TestGetSubscription(t *testing.T) {
	c := mockPaddle(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/subscriptions/sub_01h849vje9skebr9p3bx2g0mxn", r.URL.Path)
		assertPaddleRequest(t, r)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "sub_01h849vje9skebr9p3bx2g0mxn",
				"status": "active",
				"customer_id": "ctm_01h8441jn5pcwrfhwh78jqt8hk",
				"items": [{"status": "active", "quantity": 1, "recurring": true, "price": {"id": "pri_01gsz8x8sawmvhz1pv30nge1ke"}}],
				"billing_cycle": {"interval": "month", "frequency": 1},
				"created_at": "2024-04-11T15:57:24.800Z",
				"updated_at": "2024-04-11T15:57:24.800Z"
			},
			"meta": {"request_id": "req_02"}
		}`))
	})

	sub, err := c.GetSubscription(context.Background(), "sub_01h849vje9skebr9p3bx2g0mxn")
	require.NoError(t, err)
	assert.Equal(t, "active", sub.Status)
	assert.Equal(t, "pri_01gsz8x8sawmvhz1pv30nge1ke", sub.PriceID())
	assert.Equal(t, "month", sub.BillingCycle.Interval)
}

func TestGetSubscription_EmptyID(t *testing.T) {
	c := NewClient("pdl_test_key", false, nil)
	_, err := c.GetSubscription(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "subscription ID")
}

func TestCancelSubscription(t *testing.T) {
	c := mockPaddle(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/subscriptions/sub_01h849vje9skebr9p3bx2g0mxn/cancel", r.URL.Path)

		var sent map[string]any
		readJSON(t, r, &sent)
		assert.Equal(t, "next_billing_period", sent["effective_from"])

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": {
				"id": "sub_01h849vje9skebr9p3bx2g0mxn",
				"status": "active",
				"customer_id": "ctm_01h8441jn5pcwrfhwh78jqt8hk",
				"scheduled_change": {"action": "cancel", "effective_at": "2024-05-11T15:57:24.800Z", "resume_at": null},
				"created_at": "2024-04-11T15:57:24.800Z",
				"updated_at": "2024-04-12T09:00:00.000Z"
			},
			"meta": {"request_id": "req_03"}
		}`))
	})

	sub, err := c.CancelSubscription(context.Background(), "sub_01h849vje9skebr9p3bx2g0mxn")
	require.NoError(t, err)
	assert.True(t, sub.CancelScheduled())
}

func TestListSubscriptions(t *testing.T) {
	c := mockPaddle(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodGet, r.Method)
		assert.Equal(t, "/subscriptions", r.URL.Path)
		assert.Equal(t, "ctm_01h8441jn5pcwrfhwh78jqt8hk", r.URL.Query().Get("customer_id"))

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [
				{"id": "sub_02", "status": "active", "customer_id": "ctm_01h8441jn5pcwrfhwh78jqt8hk", "created_at": "2024-04-11T15:57:24.800Z", "updated_at": "2024-04-11T15:57:24.800Z"},
				{"id": "sub_01", "status": "canceled", "customer_id": "ctm_01h8441jn5pcwrfhwh78jqt8hk", "created_at": "2024-03-11T15:57:24.800Z", "updated_at": "2024-04-01T15:57:24.800Z"}
			],
			"meta": {"request_id": "req_04", "pagination": {"per_page": 50, "has_more": false, "estimated_total": 2}}
		}`))
	})

	subs, err := c.ListSubscriptions(context.Background(), "ctm_01h8441jn5pcwrfhwh78jqt8hk")
	require.NoError(t, err)
	require.Len(t, subs, 2)
	assert.Equal(t, "sub_02", subs[0].ID)
	assert.Equal(t, "canceled", subs[1].Status)
}

func TestListSubscriptions_EmptyCustomerID(t *testing.T) {
	c := NewClient("pdl_test_key", false, nil)
	_, err := c.ListSubscriptions(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "customer ID")
}

func TestCreateCustomer(t *testing.T) {
	c := mockPaddle(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/customers", r.URL.Path)
		var sent map[string]any
		readJSON(t, r, &sent)
		assert.Equal(t, "jo@example.com", sent["email"])
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data": {"id": "ctm_01hv6y1jedq4p1n0yqn5ba3ky4", "email": "jo@example.com", "status": "active"}, "meta": {"request_id": "req_05"}}`))
	})

	cust, err := c.CreateCustomer(context.Background(), "jo@example.com", "", nil)
	require.NoError(t, err)
	assert.Equal(t, "ctm_01hv6y1jedq4p1n0yqn5ba3ky4", cust.ID)
}

func TestCreateCustomer_EmptyEmail(t *testing.T) {
	c := NewClient("pdl_test_key", false, nil)
	_, err := c.CreateCustomer(context.Background(), "", "", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "email")
}

func TestNewClient_BaseURLSwitch(t *testing.T) {
	live := NewClient("k", false, nil)
	assert.Equal(t, LiveBaseURL, live.baseURL)
	sandbox := NewClient("k", true, nil)
	assert.Equal(t, SandboxBaseURL, sandbox.baseURL)
}

func TestCheckoutURLWithRedirects(t *testing.T) {
	got, err := checkoutURLWithRedirects("https://example.com/checkout", "https://example.com/ok", "")
	require.NoError(t, err)
	u, _ := url.Parse(got)
	assert.Equal(t, "https://example.com/ok", u.Query().Get("success_url"))
	assert.Empty(t, u.Query().Get("cancel_url"))

	// No checkout URL configured — nothing to hang params on.
	got, err = checkoutURLWithRedirects("", "https://example.com/ok", "")
	require.NoError(t, err)
	assert.Empty(t, got)
}
