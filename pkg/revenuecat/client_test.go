package revenuecat

import (
	"context"
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

func TestParseWebhookPayload_RealShape(t *testing.T) {
	// The actual RevenueCat webhook sends a SINGLE "event" object with an
	// api_version field — not an "events" array. Regression test for the
	// silent-parse bug that dropped every real webhook. Field names match the
	// documented schema: id, event_timestamp_ms, *_at_ms timestamps.
	body := []byte(`{
		"api_version": "1.0",
		"event": {
			"type": "INITIAL_PURCHASE",
			"id": "evt-abc",
			"event_timestamp_ms": 1753200000000,
			"store": "APP_STORE",
			"app_user_id": "0191fa87-6ed1-7022-9999-0123456789ae",
			"product_id": "com.growth.pro.monthly",
			"entitlement_ids": ["pro"],
			"period_type": "NORMAL",
			"purchased_at_ms": 1753142400000,
			"expiration_at_ms": 1755820800000,
			"transaction_id": "2000000123456789",
			"original_transaction_id": "2000000123456789"
		}
	}`)

	payload, err := ParseWebhookPayload(body)
	require.NoError(t, err)
	require.NotNil(t, payload.Event)
	assert.Equal(t, "INITIAL_PURCHASE", payload.Event.Type)
	assert.Equal(t, "0191fa87-6ed1-7022-9999-0123456789ae", payload.Event.AppUserID)

	events := payload.AllEvents()
	require.Len(t, events, 1)
	e := events[0]
	assert.Equal(t, "INITIAL_PURCHASE", e.Type)
	assert.Equal(t, "evt-abc", e.EventID)
	assert.Equal(t, int64(1753200000000), e.EventTimestampMs)
	assert.Equal(t, []string{"pro"}, e.EntitlementIDs)
	assert.False(t, e.OccurredAt().IsZero())
	assert.False(t, e.PurchasedTime().IsZero())
	assert.False(t, e.ExpirationTime().IsZero())
	assert.False(t, e.IsTrial())
}

func TestParseWebhookPayload(t *testing.T) {
	body := []byte(`{
		"events": [
			{
				"type": "INITIAL_PURCHASE",
				"store": "APP_STORE",
				"app_user_id": "0191fa87-6ed1-7022-9999-0123456789ae",
				"product_id": "com.growth.pro.monthly",
				"entitlement_ids": ["pro"],
				"purchased_at_ms": 1753142400000,
				"expiration_at_ms": 1755820800000
			},
			{
				"type": "EXPIRATION",
				"store": "PLAY_STORE",
				"app_user_id": "0191fa87-6ed1-7022-9999-0123456789ae",
				"product_id": "com.growth.pro.annual",
				"entitlement_ids": ["pro"],
				"expiration_at_ms": 1752969600000
			}
		]
	}`)

	payload, err := ParseWebhookPayload(body)
	require.NoError(t, err)
	require.Len(t, payload.Events, 2)
	assert.Equal(t, "INITIAL_PURCHASE", payload.Events[0].Type)
	assert.Equal(t, "APP_STORE", payload.Events[0].Store)
	assert.Equal(t, []string{"pro"}, payload.Events[0].EntitlementIDs)
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

func TestSubtleEqual(t *testing.T) {
	assert.True(t, subtleEqual("abc", "abc"))
	assert.False(t, subtleEqual("abc", "abd"))
	assert.False(t, subtleEqual("abc", "ab"))
	assert.False(t, subtleEqual("", "a"))
	assert.True(t, subtleEqual("", ""))
}

func TestWebhookEvent_Fields(t *testing.T) {
	// Verify all fields parse correctly from a realistic webhook payload.
	body := []byte(`{"events":[{"type":"RENEWAL","id":"evt-123","store":"APP_STORE","app_user_id":"user-1","original_app_user_id":"$RCAnonymousID:abc","aliases":["user-1","$RCAnonymousID:abc"],"product_id":"com.growth.pro.monthly","entitlement_ids":["pro"],"period_type":"NORMAL","purchased_at_ms":1753142400000,"expiration_at_ms":1755820800000,"is_trial_conversion":true,"event_timestamp_ms":1753200000000}]}`)
	payload, err := ParseWebhookPayload(body)
	require.NoError(t, err)
	require.Len(t, payload.Events, 1)
	e := payload.Events[0]
	assert.Equal(t, "RENEWAL", e.Type)
	assert.Equal(t, "user-1", e.AppUserID)
	assert.Equal(t, "$RCAnonymousID:abc", e.OriginalAppUserID)
	assert.Equal(t, []string{"user-1", "$RCAnonymousID:abc"}, e.Aliases)
	assert.Equal(t, "evt-123", e.EventID)
	assert.True(t, e.IsTrialConversion)
	assert.True(t, strings.HasPrefix(e.ProductID, "com.growth"))
}

func TestWebhookEvent_TrialAndGraceFields(t *testing.T) {
	body := []byte(`{"event":{"type":"BILLING_ISSUE","app_user_id":"user-1","period_type":"TRIAL","expiration_at_ms":1755820800000,"grace_period_expiration_at_ms":1756000000000}}`)
	payload, err := ParseWebhookPayload(body)
	require.NoError(t, err)
	e := payload.AllEvents()[0]
	assert.True(t, e.IsTrial())
	assert.False(t, e.GracePeriodExpiration().IsZero())
}
