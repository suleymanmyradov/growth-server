package billingservicelogic

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/pkg/revenuecat"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func strPtr(s string) *string { return &s }

func rcTestLogic(m *mockBilling) *HandleRevenueCatWebhookLogic {
	cfg := config.Config{}
	cfg.Billing.RevenueCat.Enabled = true
	cfg.Billing.RevenueCat.WebhookSecret = "secret"
	return &HandleRevenueCatWebhookLogic{
		ctx: context.Background(),
		svcCtx: &svc.ServiceContext{
			Config: cfg,
			Repo:   &repository.Repository{Billing: m},
		},
		Logger:       logx.WithContext(context.Background()),
		testTxRunner: noopTxRunner{},
	}
}

func rcWebhookBody(events ...map[string]any) []byte {
	// Emit RevenueCat's real envelope — {"api_version":"1.0","event":{...}} —
	// for the common single-event case; multiple events use the tolerated
	// array shape so batch behavior stays covered.
	if len(events) == 1 {
		payload := map[string]any{"api_version": "1.0", "event": events[0]}
		b, _ := json.Marshal(payload)
		return b
	}
	payload := map[string]any{"events": events}
	b, _ := json.Marshal(payload)
	return b
}

// rcEvent builds an event map using RevenueCat's real field names
// (id, event_timestamp_ms, purchased_at_ms, expiration_at_ms, ...).
func rcEvent(userID uuid.UUID, eventType string, extra map[string]any) map[string]any {
	now := time.Now()
	evt := map[string]any{
		"type":               eventType,
		"id":                 "evt_" + eventType + "_" + uuid.NewString()[:8],
		"event_timestamp_ms": now.UnixMilli(),
		"store":              "APP_STORE",
		"app_user_id":        userID.String(),
		"product_id":         "com.growth.pro.monthly",
		"period_type":        "NORMAL",
		"purchased_at_ms":    now.Add(-24 * time.Hour).UnixMilli(),
		"expiration_at_ms":   now.Add(30 * 24 * time.Hour).UnixMilli(),
	}
	for k, v := range extra {
		if v == nil {
			delete(evt, k)
			continue
		}
		evt[k] = v
	}
	return evt
}

func rcTestPlans() map[string]db.Plan {
	return map[string]db.Plan{
		"pro":  {ID: paddleProPlanID, Code: "pro"},
		"free": {ID: paddleFreePlanID, Code: "free"},
	}
}

func rcMock() *mockBilling {
	m := newMockBilling()
	m.plans = rcTestPlans()
	return m
}

// ─── Envelope-level tests ───────────────────────────────────────────────────

func TestHandleRevenueCatWebhook_NotConfigured(t *testing.T) {
	l := rcTestLogic(newMockBilling())
	l.svcCtx.Config.Billing.RevenueCat.Enabled = false

	_, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       rcWebhookBody(),
		Authorization: "Bearer secret",
	})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestHandleRevenueCatWebhook_InvalidSignature(t *testing.T) {
	l := rcTestLogic(newMockBilling())

	_, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       rcWebhookBody(),
		Authorization: "Bearer wrong-secret",
	})
	require.Error(t, err)
	assert.Equal(t, codes.Unauthenticated, status.Code(err))
}

func TestHandleRevenueCatWebhook_EmptyBody(t *testing.T) {
	l := rcTestLogic(newMockBilling())

	_, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       nil,
		Authorization: "Bearer secret",
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHandleRevenueCatWebhook_NoEvents(t *testing.T) {
	l := rcTestLogic(newMockBilling())

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       rcWebhookBody(),
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
}

// ─── Purchase / renewal events ──────────────────────────────────────────────

func TestHandleRevenueCatWebhook_InitialPurchase(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	l := rcTestLogic(m)

	purchased := time.Now().Add(-time.Hour)
	expires := time.Now().Add(30 * 24 * time.Hour)
	body := rcWebhookBody(rcEvent(userID, "INITIAL_PURCHASE", map[string]any{
		"purchased_at_ms":  purchased.UnixMilli(),
		"expiration_at_ms": expires.UnixMilli(),
		"entitlement_ids":  []any{"pro"},
	}))

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	// Provider state written for 'revenuecat' with real *_at_ms fields parsed.
	require.Len(t, m.upsertCalls, 1)
	p := m.upsertCalls[0]
	assert.Equal(t, "revenuecat", p.Provider)
	assert.Equal(t, "active", p.Status)
	assert.Equal(t, userID, p.UserID)
	require.NotNil(t, p.BillingInterval)
	assert.Equal(t, "monthly", *p.BillingInterval)
	assert.False(t, p.CancelAtPeriodEnd)
	assert.WithinDuration(t, purchased, p.CurrentPeriodStart.Time, time.Second)
	assert.WithinDuration(t, expires, p.CurrentPeriodEnd.Time, time.Second)
	assert.Equal(t, userID.String(), *p.ProviderCustomerID)

	// Merged projection grants pro.
	merged := m.lastMerged(userID)
	assert.Equal(t, "active", merged.Status)
	assert.Equal(t, paddleProPlanID, merged.PlanID)
	require.NotNil(t, merged.RevenuecatCustomerID)
	assert.Equal(t, userID.String(), *merged.RevenuecatCustomerID)

	// Funnel event recorded.
	require.Len(t, m.upgradeEvents, 1)
	assert.Equal(t, "checkout_completed", m.upgradeEvents[0].EventType)
}

func TestHandleRevenueCatWebhook_Renewal(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	m.seedState(db.SubscriptionProviderState{
		UserID:            userID,
		Provider:          "revenuecat",
		Status:            "active",
		CurrentPeriodEnd:  pgtype.Timestamptz{Time: time.Now().Add(24 * time.Hour), Valid: true},
		CancelAtPeriodEnd: true, // was set to cancel — renewal clears it
	})
	l := rcTestLogic(m)

	body := rcWebhookBody(rcEvent(userID, "RENEWAL", nil))
	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "active", m.upsertCalls[0].Status)
	assert.False(t, m.upsertCalls[0].CancelAtPeriodEnd)
}

func TestHandleRevenueCatWebhook_TrialPeriodType(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	l := rcTestLogic(m)

	expires := time.Now().Add(7 * 24 * time.Hour)
	body := rcWebhookBody(rcEvent(userID, "INITIAL_PURCHASE", map[string]any{
		"period_type":      "TRIAL",
		"expiration_at_ms": expires.UnixMilli(),
	}))

	_, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "trialing", m.upsertCalls[0].Status)
	assert.True(t, m.upsertCalls[0].TrialEnd.Valid)
	merged := m.lastMerged(userID)
	assert.Equal(t, "trialing", merged.Status)
	assert.Equal(t, paddleProPlanID, merged.PlanID)
}

func TestHandleRevenueCatWebhook_AnnualInterval(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	l := rcTestLogic(m)

	body := rcWebhookBody(rcEvent(userID, "RENEWAL", map[string]any{
		"product_id": "com.growth.pro.annual",
	}))

	_, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	require.Len(t, m.upsertCalls, 1)
	require.NotNil(t, m.upsertCalls[0].BillingInterval)
	assert.Equal(t, "annual", *m.upsertCalls[0].BillingInterval)
}

// ─── Cancellation / expiration / billing issues (B5) ────────────────────────

func TestHandleRevenueCatWebhook_CancellationUnsubscribe(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	periodEnd := time.Now().Add(20 * 24 * time.Hour)
	m.seedState(db.SubscriptionProviderState{
		UserID:           userID,
		Provider:         "revenuecat",
		Status:           "active",
		BillingInterval:  strPtr("monthly"),
		CurrentPeriodEnd: pgtype.Timestamptz{Time: periodEnd, Valid: true},
	})
	l := rcTestLogic(m)

	body := rcWebhookBody(rcEvent(userID, "CANCELLATION", map[string]any{
		"cancel_reason":    "UNSUBSCRIBE",
		"expiration_at_ms": periodEnd.UnixMilli(),
	}))

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	// Still active until period end; the flag carries the pending cancel.
	require.Len(t, m.upsertCalls, 1)
	assert.True(t, m.upsertCalls[0].CancelAtPeriodEnd)
	assert.Equal(t, "active", m.upsertCalls[0].Status)
	merged := m.lastMerged(userID)
	assert.Equal(t, "active", merged.Status)
	assert.Equal(t, paddleProPlanID, merged.PlanID)
}

func TestHandleRevenueCatWebhook_CancellationRefundRevokes(t *testing.T) {
	// B5: cancel_reason=CUSTOMER_SUPPORT is the store-refund path — access
	// ends now, not at period end, and the event is recorded.
	userID := uuid.New()
	m := rcMock()
	m.seedState(db.SubscriptionProviderState{
		UserID:           userID,
		Provider:         "revenuecat",
		Status:           "active",
		BillingInterval:  strPtr("monthly"),
		CurrentPeriodEnd: pgtype.Timestamptz{Time: time.Now().Add(20 * 24 * time.Hour), Valid: true},
	})
	l := rcTestLogic(m)

	body := rcWebhookBody(rcEvent(userID, "CANCELLATION", map[string]any{
		"cancel_reason": "CUSTOMER_SUPPORT",
	}))

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	assert.Equal(t, "expired", m.rcState(userID).Status)
	merged := m.lastMerged(userID)
	assert.Equal(t, "expired", merged.Status)
	assert.Equal(t, paddleFreePlanID, merged.PlanID)

	require.Len(t, m.upgradeEvents, 1)
	assert.Equal(t, "subscription_refunded", m.upgradeEvents[0].EventType)
	assert.Equal(t, "revenuecat_webhook", m.upgradeEvents[0].Surface)
}

func TestHandleRevenueCatWebhook_CancellationBillingError(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	m.seedState(db.SubscriptionProviderState{
		UserID:           userID,
		Provider:         "revenuecat",
		Status:           "active",
		CurrentPeriodEnd: pgtype.Timestamptz{Time: time.Now().Add(5 * 24 * time.Hour), Valid: true},
	})
	l := rcTestLogic(m)

	graceEnd := time.Now().Add(16 * 24 * time.Hour)
	body := rcWebhookBody(rcEvent(userID, "CANCELLATION", map[string]any{
		"cancel_reason":                 "BILLING_ERROR",
		"grace_period_expiration_at_ms": graceEnd.UnixMilli(),
		"expiration_at_ms":              time.Now().Add(5 * 24 * time.Hour).UnixMilli(),
	}))

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "past_due", m.upsertCalls[0].Status)
	assert.WithinDuration(t, graceEnd, m.upsertCalls[0].CurrentPeriodEnd.Time, time.Second)
}

func TestHandleRevenueCatWebhook_BillingIssue(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	m.seedState(db.SubscriptionProviderState{
		UserID:           userID,
		Provider:         "revenuecat",
		Status:           "active",
		CurrentPeriodEnd: pgtype.Timestamptz{Time: time.Now().Add(5 * 24 * time.Hour), Valid: true},
	})
	l := rcTestLogic(m)

	// Grace extends past the nominal period end — paid access ends at grace.
	graceEnd := time.Now().Add(14 * 24 * time.Hour)
	body := rcWebhookBody(rcEvent(userID, "BILLING_ISSUE", map[string]any{
		"grace_period_expiration_at_ms": graceEnd.UnixMilli(),
		"expiration_at_ms":              time.Now().Add(5 * 24 * time.Hour).UnixMilli(),
	}))

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "past_due", m.upsertCalls[0].Status)
	assert.WithinDuration(t, graceEnd, m.upsertCalls[0].CurrentPeriodEnd.Time, time.Second)
}

func TestHandleRevenueCatWebhook_Expiration(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	m.seedState(db.SubscriptionProviderState{
		UserID:           userID,
		Provider:         "revenuecat",
		Status:           "active",
		CurrentPeriodEnd: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
	})
	l := rcTestLogic(m)

	body := rcWebhookBody(rcEvent(userID, "EXPIRATION", map[string]any{
		"store":            "PLAY_STORE",
		"expiration_at_ms": time.Now().Add(-time.Hour).UnixMilli(),
	}))

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "expired", m.upsertCalls[0].Status)
	merged := m.lastMerged(userID)
	assert.Equal(t, "expired", merged.Status)
	assert.Equal(t, paddleFreePlanID, merged.PlanID)
}

func TestHandleRevenueCatWebhook_Paused(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	m.seedState(db.SubscriptionProviderState{
		UserID:           userID,
		Provider:         "revenuecat",
		Status:           "active",
		CurrentPeriodEnd: pgtype.Timestamptz{Time: time.Now().Add(20 * 24 * time.Hour), Valid: true},
	})
	l := rcTestLogic(m)

	body := rcWebhookBody(rcEvent(userID, "SUBSCRIPTION_PAUSED", map[string]any{
		"store":             "PLAY_STORE",
		"auto_resume_at_ms": time.Now().Add(60 * 24 * time.Hour).UnixMilli(),
	}))

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "paused", m.upsertCalls[0].Status)
	merged := m.lastMerged(userID)
	assert.Equal(t, "paused", merged.Status)
}

// ─── B2: out-of-order events ────────────────────────────────────────────────

func TestHandleRevenueCatWebhook_StaleEventIgnored(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	lastEvent := time.Now()
	m.seedState(db.SubscriptionProviderState{
		UserID:           userID,
		Provider:         "revenuecat",
		Status:           "active",
		CurrentPeriodEnd: pgtype.Timestamptz{Time: time.Now().Add(20 * 24 * time.Hour), Valid: true},
		LastEventAt:      pgtype.Timestamptz{Time: lastEvent, Valid: true},
	})
	l := rcTestLogic(m)

	// A delayed cancellation predating the last applied event.
	body := rcWebhookBody(rcEvent(userID, "CANCELLATION", map[string]any{
		"event_timestamp_ms": lastEvent.Add(-time.Hour).UnixMilli(),
		"cancel_reason":      "UNSUBSCRIBE",
	}))

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls, "stale event must not write provider state")
	assert.Equal(t, "active", m.rcState(userID).Status)
}

func TestHandleRevenueCatWebhook_NewerEventApplies(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	m.seedState(db.SubscriptionProviderState{
		UserID:           userID,
		Provider:         "revenuecat",
		Status:           "active",
		CurrentPeriodEnd: pgtype.Timestamptz{Time: time.Now().Add(20 * 24 * time.Hour), Valid: true},
		LastEventAt:      pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
	})
	l := rcTestLogic(m)

	body := rcWebhookBody(rcEvent(userID, "CANCELLATION", map[string]any{
		"event_timestamp_ms": time.Now().UnixMilli(),
		"cancel_reason":      "UNSUBSCRIBE",
	}))

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	require.Len(t, m.upsertCalls, 1)
	assert.True(t, m.upsertCalls[0].CancelAtPeriodEnd)
}

func TestHandleRevenueCatWebhook_NoTimestampAppliesButKeepsWatermark(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	watermark := time.Now()
	m.seedState(db.SubscriptionProviderState{
		UserID:           userID,
		Provider:         "revenuecat",
		Status:           "active",
		CurrentPeriodEnd: pgtype.Timestamptz{Time: time.Now().Add(20 * 24 * time.Hour), Valid: true},
		LastEventAt:      pgtype.Timestamptz{Time: watermark, Valid: true},
	})
	l := rcTestLogic(m)

	// An event without event_timestamp_ms applies (can't prove staleness)
	// but must not regress the watermark.
	body := rcWebhookBody(rcEvent(userID, "CANCELLATION", map[string]any{
		"event_timestamp_ms": nil,
		"cancel_reason":      "UNSUBSCRIBE",
	}))

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	require.Len(t, m.upsertCalls, 1)
	assert.WithinDuration(t, watermark, m.rcState(userID).LastEventAt.Time, time.Second)
}

// ─── B1: cross-provider merge ───────────────────────────────────────────────

func TestHandleRevenueCatWebhook_RCExpiryDoesNotKillActivePaddle(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	m.seedState(db.SubscriptionProviderState{
		UserID:           userID,
		Provider:         "revenuecat",
		Status:           "active",
		CurrentPeriodEnd: pgtype.Timestamptz{Time: time.Now().Add(-time.Hour), Valid: true},
	})
	m.seedState(db.SubscriptionProviderState{
		UserID:             userID,
		Provider:           "paddle",
		Status:             "active",
		CurrentPeriodEnd:   pgtype.Timestamptz{Time: time.Now().Add(40 * 24 * time.Hour), Valid: true},
		CurrentPeriodStart: pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true},
	})
	l := rcTestLogic(m)

	body := rcWebhookBody(rcEvent(userID, "EXPIRATION", nil))
	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	assert.Equal(t, "expired", m.rcState(userID).Status)
	// Merged projection still grants pro via Paddle.
	merged := m.lastMerged(userID)
	assert.Equal(t, "active", merged.Status)
	assert.Equal(t, paddleProPlanID, merged.PlanID)
}

// ─── Idempotency / error semantics ──────────────────────────────────────────

func TestHandleRevenueCatWebhook_DuplicateEventSkipped(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	m.allProcessed = true
	l := rcTestLogic(m)

	body := rcWebhookBody(rcEvent(userID, "INITIAL_PURCHASE", map[string]any{
		"id": "evt-duplicate-1",
	}))

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls)
}

func TestHandleRevenueCatWebhook_InvalidUserID(t *testing.T) {
	m := rcMock()
	l := rcTestLogic(m)

	evt := rcEvent(uuid.New(), "INITIAL_PURCHASE", nil)
	evt["app_user_id"] = "not-a-uuid"
	body := rcWebhookBody(evt)

	// Permanent failure → marked processed, response still OK.
	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls)
}

func TestHandleRevenueCatWebhook_RetryableFailureReturnsError(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	m.upsertErr = errors.New("database connection lost")
	l := rcTestLogic(m)

	body := rcWebhookBody(rcEvent(userID, "INITIAL_PURCHASE", nil))

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.Error(t, err, "retryable failure must return error so RevenueCat retries")
	assert.Nil(t, resp)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestHandleRevenueCatWebhook_MarkProcessedFailureReturnsError(t *testing.T) {
	userID := uuid.New()
	m := rcMock()
	m.markProcessedErr = errors.New("db error marking processed")
	l := rcTestLogic(m)

	body := rcWebhookBody(rcEvent(userID, "INITIAL_PURCHASE", nil))

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.Error(t, err, "mark-processed failure must return error so RevenueCat retries")
	assert.Nil(t, resp)
	assert.Equal(t, codes.Internal, status.Code(err))
}

func TestDeriveEventID(t *testing.T) {
	evt := revenuecat.WebhookEvent{
		Type:             "INITIAL_PURCHASE",
		AppUserID:        "user-1",
		ProductID:        "com.growth.pro.monthly",
		EventTimestampMs: 1753228800000,
		TransactionID:    "txn-1",
	}
	id1 := deriveEventID(evt)
	assert.Equal(t, id1, deriveEventID(evt), "derived event ID should be deterministic")

	evt.ProductID = "com.growth.pro.annual"
	assert.NotEqual(t, id1, deriveEventID(evt))
}
