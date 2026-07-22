package stripe

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stripe/stripe-go/v82/webhook"
)

// signPayload is a helper that creates a real Stripe-signed webhook payload
// using the SDK's test helper, returning the raw payload and the signature
// header string that VerifyWebhookSignature expects.
func signPayload(t *testing.T, payload []byte, secret string) ([]byte, string) {
	t.Helper()
	signed := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload:   payload,
		Secret:    secret,
		Timestamp: time.Now(),
	})
	return payload, signed.Header
}

// TestVerifyWebhookSignature_ValidPayload creates a real signed webhook
// payload using the Stripe SDK's webhook.Sign helper, then verifies it
// through our Client.VerifyWebhookSignature method. This tests the actual
// cryptographic verification path end-to-end (no mocking).
func TestVerifyWebhookSignature_ValidPayload(t *testing.T) {
	secret := "whsec_test_secret_12345"
	c := NewClient("sk_test_dummy")

	payload := map[string]interface{}{
		"id":          "evt_test_001",
		"type":        "checkout.session.completed",
		"api_version": "2025-08-27.basil",
		"data": map[string]interface{}{
			"object": map[string]interface{}{
				"id":           "cs_test_001",
				"customer":     "cus_test_001",
				"subscription": "sub_test_001",
			},
		},
	}
	rawPayload, err := json.Marshal(payload)
	require.NoError(t, err)

	raw, header := signPayload(t, rawPayload, secret)

	eventType, err := c.VerifyWebhookSignature(raw, header, secret)
	require.NoError(t, err)
	assert.Equal(t, "checkout.session.completed", eventType)
}

// TestVerifyWebhookSignature_InvalidSignature verifies that a payload
// signed with a different secret is rejected.
func TestVerifyWebhookSignature_InvalidSignature(t *testing.T) {
	secret := "whsec_test_secret_12345"
	c := NewClient("sk_test_dummy")

	payload := []byte(`{"id":"evt_test_002","type":"checkout.session.completed"}`)

	// Sign with a different secret than we verify with — should fail.
	raw, header := signPayload(t, payload, "whsec_wrong_secret_99999")

	eventType, err := c.VerifyWebhookSignature(raw, header, secret)
	assert.Error(t, err)
	assert.Empty(t, eventType)
}

// TestVerifyWebhookSignature_TamperedPayload verifies that a payload
// modified after signing is rejected.
func TestVerifyWebhookSignature_TamperedPayload(t *testing.T) {
	secret := "whsec_test_secret_12345"
	c := NewClient("sk_test_dummy")

	payload := []byte(`{"id":"evt_test_003","type":"checkout.session.completed"}`)
	_, header := signPayload(t, payload, secret)

	// Tamper with the payload after signing.
	tampered := []byte(`{"id":"evt_HACKED","type":"checkout.session.completed"}`)
	eventType, err := c.VerifyWebhookSignature(tampered, header, secret)
	assert.Error(t, err)
	assert.Empty(t, eventType)
}

// TestVerifyWebhookSignature_EmptySignature verifies that an empty
// signature header is rejected.
func TestVerifyWebhookSignature_EmptySignature(t *testing.T) {
	c := NewClient("sk_test_dummy")

	eventType, err := c.VerifyWebhookSignature([]byte(`{}`), "", "whsec_test")
	assert.Error(t, err)
	assert.Empty(t, eventType)
}

// TestVerifyWebhookSignature_APIVersionMismatch verifies that events
// with a newer API version are still accepted (we set
// IgnoreAPIVersionMismatch: true). This is the bug we hit in production —
// Stripe dashboard sends 2026-06-24.dahlia but stripe-go 82.5.1 pins
// 2025-08-27.basil.
func TestVerifyWebhookSignature_APIVersionMismatch(t *testing.T) {
	secret := "whsec_test_secret_12345"
	c := NewClient("sk_test_dummy")

	// Use a future/unknown API version that the SDK doesn't know about.
	payload := map[string]interface{}{
		"id":          "evt_test_004",
		"type":        "invoice.paid",
		"api_version": "2099-01-01.future",
		"data": map[string]interface{}{
			"object": map[string]interface{}{"id": "in_test_001"},
		},
	}
	rawPayload, err := json.Marshal(payload)
	require.NoError(t, err)

	raw, header := signPayload(t, rawPayload, secret)

	// Should still pass — we ignore API version mismatch.
	eventType, err := c.VerifyWebhookSignature(raw, header, secret)
	require.NoError(t, err)
	assert.Equal(t, "invoice.paid", eventType)
}
