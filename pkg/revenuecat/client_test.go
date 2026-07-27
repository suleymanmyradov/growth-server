package revenuecat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func mockRevenueCat(t *testing.T, handler http.HandlerFunc) (*Client, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient("sk_test_key", "proj_test", nil, WithBaseURL(srv.URL))
	return c, srv
}

func TestGetCustomerEntitlements_HappyPath(t *testing.T) {
	c, _ := mockRevenueCat(t, func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "Bearer sk_test_key", r.Header.Get("Authorization"))
		assert.Contains(t, r.URL.Path, "/customers/user-123/entitlements")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"items": [
				{"id": "pro", "product_id": "com.growth.pro.monthly", "store": "APP_STORE", "is_active": true},
				{"id": "legacy", "product_id": "com.growth.legacy", "store": "PLAY_STORE", "is_active": false}
			]
		}`))
	})

	ents, err := c.GetCustomerEntitlements(context.Background(), "user-123")
	require.NoError(t, err)
	require.Len(t, ents, 2)
	assert.Equal(t, "pro", ents[0].ID)
	assert.True(t, ents[0].IsActive)
	assert.False(t, ents[1].IsActive)
}

func TestGetCustomerEntitlements_HasPro(t *testing.T) {
	ents := []Entitlement{
		{ID: "pro", IsActive: true},
		{ID: "legacy", IsActive: false},
	}
	assert.True(t, HasProEntitlement(ents))

	ents[0].IsActive = false
	assert.False(t, HasProEntitlement(ents))

	assert.False(t, HasProEntitlement([]Entitlement{{ID: "free", IsActive: true}}))
}

func TestGetCustomerEntitlements_HTTPError(t *testing.T) {
	c, _ := mockRevenueCat(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"customer not found"}`))
	})

	_, err := c.GetCustomerEntitlements(context.Background(), "unknown")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "404")
}

func TestGetCustomerEntitlements_MissingConfig(t *testing.T) {
	c := NewClient("", "proj", nil)
	_, err := c.GetCustomerEntitlements(context.Background(), "user-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key")

	c2 := NewClient("sk", "", nil)
	_, err = c2.GetCustomerEntitlements(context.Background(), "user-1")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "project ID")
}

func TestGetCustomerEntitlements_EmptyCustomerID(t *testing.T) {
	c := NewClient("sk", "proj", nil)
	_, err := c.GetCustomerEntitlements(context.Background(), "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "customer ID is required")
}

func TestParseWebhookPayload(t *testing.T) {
	body := []byte(`{
		"events": [
			{
				"type": "INITIAL_PURCHASE",
				"store": "APP_STORE",
				"app_user_id": "0191fa87-6ed1-7022-9999-0123456789ae",
				"product_id": "com.growth.pro.monthly",
				"entitlement_id": "pro",
				"period_start_at": "2025-07-22T00:00:00Z",
				"expiration_at": "2025-08-22T00:00:00Z"
			},
			{
				"type": "EXPIRATION",
				"store": "PLAY_STORE",
				"app_user_id": "0191fa87-6ed1-7022-9999-0123456789ae",
				"product_id": "com.growth.pro.annual",
				"entitlement_id": "pro",
				"expiration_at": "2025-07-20T00:00:00Z"
			}
		]
	}`)

	payload, err := ParseWebhookPayload(body)
	require.NoError(t, err)
	require.Len(t, payload.Events, 2)
	assert.Equal(t, "INITIAL_PURCHASE", payload.Events[0].Type)
	assert.Equal(t, "APP_STORE", payload.Events[0].Store)
	assert.Equal(t, "pro", payload.Events[0].EntitlementID)
	assert.Equal(t, "EXPIRATION", payload.Events[1].Type)
}

func TestParseWebhookPayload_InvalidJSON(t *testing.T) {
	_, err := ParseWebhookPayload([]byte(`{not json`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse webhook payload")
}

func TestVerifyWebhookSignature(t *testing.T) {
	secret := "rc_webhook_secret_abc123"
	assert.True(t, VerifyWebhookSignature("Bearer "+secret, secret))
	assert.False(t, VerifyWebhookSignature("Bearer wrong", secret))
	assert.False(t, VerifyWebhookSignature("Bearer "+secret, "wrong"))
	assert.False(t, VerifyWebhookSignature("", secret))
	assert.False(t, VerifyWebhookSignature("Bearer "+secret, ""))
	assert.False(t, VerifyWebhookSignature("Token "+secret, secret))
}

func TestPostSubscription_HappyPath(t *testing.T) {
	var sentBody string
	c, _ := mockRevenueCat(t, func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		sentBody = string(buf)
		assert.Contains(t, r.URL.Path, "/customers/user-123/purchases")
		w.WriteHeader(http.StatusCreated)
	})

	err := c.PostSubscription(context.Background(), "user-123", PostSubscriptionRequest{
		ProductID:    "com.growth.pro.monthly",
		Store:        "STRIPE",
		Price:        999,
		Currency:     "USD",
		PurchaseDate: "2025-07-22T00:00:00Z",
	})
	require.NoError(t, err)

	var sent PostSubscriptionRequest
	require.NoError(t, json.Unmarshal([]byte(sentBody), &sent))
	assert.Equal(t, "com.growth.pro.monthly", sent.ProductID)
	assert.Equal(t, "STRIPE", sent.Store)
	assert.Equal(t, int64(999), sent.Price)
}

func TestPostSubscription_HTTPError(t *testing.T) {
	c, _ := mockRevenueCat(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"invalid product"}`))
	})

	err := c.PostSubscription(context.Background(), "user-123", PostSubscriptionRequest{
		ProductID: "bad",
		Store:     "STRIPE",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

func TestPostSubscription_MissingConfig(t *testing.T) {
	c := NewClient("", "proj", nil)
	err := c.PostSubscription(context.Background(), "user-1", PostSubscriptionRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "API key")
}

func TestSubtleEqual(t *testing.T) {
	assert.True(t, subtleEqual("abc", "abc"))
	assert.False(t, subtleEqual("abc", "abd"))
	assert.False(t, subtleEqual("abc", "ab"))
	assert.False(t, subtleEqual("", "a"))
	assert.True(t, subtleEqual("", ""))
}

func TestWebhookEvent_Fields(t *testing.T) {
	// Verify all fields parse correctly from a realistic webhook payload.
	body := []byte(`{"events":[{"type":"RENEWAL","store":"APP_STORE","app_user_id":"user-1","original_app_user_id":"$RCAnonymousID:abc","product_id":"com.growth.pro.monthly","entitlement_id":"pro","period_start_at":"2025-07-22T00:00:00Z","expiration_at":"2025-08-22T00:00:00Z","event_id":"evt-123"}]}`)
	payload, err := ParseWebhookPayload(body)
	require.NoError(t, err)
	require.Len(t, payload.Events, 1)
	e := payload.Events[0]
	assert.Equal(t, "RENEWAL", e.Type)
	assert.Equal(t, "user-1", e.AppUserID)
	assert.Equal(t, "$RCAnonymousID:abc", e.OriginalAppUserID)
	assert.Equal(t, "evt-123", e.EventID)
	assert.True(t, strings.HasPrefix(e.ProductID, "com.growth"))
}
