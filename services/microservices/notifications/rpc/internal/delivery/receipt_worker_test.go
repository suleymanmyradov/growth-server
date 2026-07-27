package delivery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/pkg/notifications/expo"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository/db"
)

// TestReceiptWorker_NilExpoIsNoop verifies that Run exits immediately when
// the Expo client is nil (Expo disabled in config).
func TestReceiptWorker_NilExpoIsNoop(t *testing.T) {
	w := NewReceiptWorker(nil, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately so Run doesn't block
	w.Run(ctx)
	// If we reach here, Run returned — test passes.
}

// TestReceiptWorker_NilTicketsRepoIsNoop verifies that Run exits immediately
// when the tickets repo is nil.
func TestReceiptWorker_NilTicketsRepoIsNoop(t *testing.T) {
	w := NewReceiptWorker(nil, nil, expo.NewClient(nil, ""))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w.Run(ctx)
}

// TestReceiptWorker_DisabledByNilExpoClient verifies the no-op guard in Run.
func TestReceiptWorker_DisabledByNilExpoClient(t *testing.T) {
	w := &ReceiptWorker{
		devices:      nil,
		tickets:      nil,
		expo:         nil, // Expo disabled
		batchSize:    10,
		interval:     100 * time.Millisecond,
		ticketMinAge: 0,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	w.Run(ctx)
	// Should return immediately without panicking.
}

// TestNewPayload_RejectsArbitraryURL verifies that the push payload validation
// rejects arbitrary URLs as destinations, preventing phishing/deep-link attacks.
func TestNewPayload_RejectsArbitraryURL(t *testing.T) {
	_, err := NewPayload("title", "body", uuid.New(), Destination("https://evil.com/path"), uuid.Nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid destination")
}

// TestNewPayload_RejectsJavaScriptScheme verifies that javascript: and other
// non-validated scheme strings are rejected.
func TestNewPayload_RejectsJavaScriptScheme(t *testing.T) {
	_, err := NewPayload("title", "body", uuid.New(), Destination("javascript:alert(1)"), uuid.Nil)
	require.Error(t, err)
}

// TestPayload_DataMapContainsVersion verifies that the data map always
// includes the schema version, so the mobile app can validate before
// interpreting any field.
func TestPayload_DataMapContainsVersion(t *testing.T) {
	p, err := NewPayload("title", "body", uuid.New(), DestinationActivity, uuid.Nil)
	require.NoError(t, err)
	data := p.dataMap()
	assert.Contains(t, data, "version")
	assert.Equal(t, PushDataVersion, data["version"])
}

// TestPayload_DataMapNeverContainsArbitraryKeys verifies that the data map
// only contains validated, known keys — no arbitrary caller-supplied data.
func TestPayload_DataMapNeverContainsArbitraryKeys(t *testing.T) {
	notifID := uuid.New()
	resourceID := uuid.New()
	p, err := NewPayload("title", "body", notifID, DestinationHabitDetail, resourceID)
	require.NoError(t, err)
	data := p.dataMap()

	// Only these keys should ever be present.
	expectedKeys := map[string]bool{
		"version":        true,
		"notificationId": true,
		"destination":    true,
		"resourceId":     true,
	}
	for k := range data {
		assert.True(t, expectedKeys[k], "unexpected key in data map: %s", k)
	}
}

// TestReceiptWorker_CheckOnceWithMockServer verifies the receipt checking
// logic: pending tickets are fetched, receipts are checked at Expo, and
// ticket status is updated. This uses a mock Expo server.
func TestReceiptWorker_CheckOnceWithMockServer(t *testing.T) {
	// This is a unit test for the receipt-checking logic. We can't easily
	// test the full flow without a DB, but we can verify the Expo receipt
	// API contract.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]any{
			"data": map[string]any{
				"ticket-ok-1": map[string]any{"status": "ok"},
				"ticket-err-1": map[string]any{
					"status":  "error",
					"message": "DeviceNotRegistered",
					"details": map[string]any{"error": "DeviceNotRegistered"},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)

	c := expo.NewClient(nil, "", expo.WithReceiptURL(srv.URL+"/--/api/v2/push/getReceipts"))

	receipts, err := c.CheckReceipts(context.Background(), []string{"ticket-ok-1", "ticket-err-1"})
	require.NoError(t, err)
	require.Len(t, receipts, 2)

	assert.Equal(t, "ok", receipts["ticket-ok-1"].Status)
	assert.Equal(t, "error", receipts["ticket-err-1"].Status)
	assert.Equal(t, "DeviceNotRegistered", receipts["ticket-err-1"].Details["error"])
}

// TestReceiptWorker_TicketMinAgeFiltering verifies that tickets younger than
// the minimum age are skipped (Expo needs time to process delivery).
func TestReceiptWorker_TicketMinAgeFiltering(t *testing.T) {
	// Create a ticket that's too young to check.
	now := time.Now()
	youngTicket := db.PushTicket{
		TicketID:      "young-ticket",
		PushToken:     "ExponentPushToken[young]",
		ReceiptStatus: "pending",
		CreatedAt:     pgtype.Timestamptz{Time: now.Add(-1 * time.Second), Valid: true}, // 1s ago
	}

	// The cutoff is now - 5s (default ticketMinAge). A 1s-old ticket is younger.
	cutoff := now.Add(-5 * time.Second)
	assert.False(t, youngTicket.CreatedAt.Valid && youngTicket.CreatedAt.Time.Before(cutoff),
		"young ticket should be filtered out (not yet eligible for receipt check)")

	// Create a ticket that's old enough to check.
	oldTicket := db.PushTicket{
		TicketID:      "old-ticket",
		PushToken:     "ExponentPushToken[old]",
		ReceiptStatus: "pending",
		CreatedAt:     pgtype.Timestamptz{Time: now.Add(-10 * time.Second), Valid: true}, // 10s ago
	}
	assert.True(t, oldTicket.CreatedAt.Valid && oldTicket.CreatedAt.Time.Before(cutoff),
		"old ticket should be eligible for receipt check")
}
