package billingservicelogic

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const paddleTestSecret = "pdl_ntfset_testsecret"

var (
	paddleTestUserID    = uuid.MustParse("0191fa87-6ed1-7022-9999-0123456789ae")
	paddleTestOtherUser = uuid.MustParse("0191fa87-6ed1-7022-9999-0123456789bb")
	paddleProPlanID     = uuid.MustParse("0191fa87-0000-7000-8000-0000000000a1")
	paddleFreePlanID    = uuid.MustParse("0191fa87-0000-7000-8000-0000000000f1")
	paddleTestPlans     = map[string]db.Plan{
		"pro":  {ID: paddleProPlanID, Code: "pro"},
		"free": {ID: paddleFreePlanID, Code: "free"},
	}
	paddleTestCustomerID = "ctm_01h8441jn5pcwrfhwh78jqt8hk"
	paddleTestSubID      = "sub_01h849vje9skebr9p3bx2g0mxn"
	paddleTestTxnID      = "txn_01hgk505qd8mm8yt0yvk1b6w8x"
)

// paddleTestLogic builds the logic with a configured webhook secret and the
// mock repo / no-op tx runner.
func paddleTestLogic(m *mockBilling) *HandlePaddleWebhookLogic {
	cfg := config.Config{}
	cfg.Billing.Paddle.Enabled = true
	cfg.Billing.Paddle.WebhookSecret = paddleTestSecret
	return &HandlePaddleWebhookLogic{
		ctx: context.Background(),
		svcCtx: &svc.ServiceContext{
			Config: cfg,
			Repo:   &repository.Repository{Billing: m},
		},
		Logger:       logx.WithContext(context.Background()),
		testTxRunner: noopTxRunner{},
	}
}

// paddleSign produces a valid Paddle-Signature header for a body.
func paddleSign(secret string, body []byte) string {
	ts := fmt.Sprintf("%d", time.Now().Unix())
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(ts + ":" + string(body)))
	return fmt.Sprintf("ts=%s;h1=%s", ts, hex.EncodeToString(mac.Sum(nil)))
}

// paddleBodyAt builds a signed event envelope with an explicit occurred_at.
func paddleBodyAt(eventType, eventID string, occurredAt time.Time, data map[string]any) ([]byte, string) {
	env := map[string]any{
		"event_id":        eventID,
		"event_type":      eventType,
		"occurred_at":     occurredAt.UTC().Format(time.RFC3339),
		"notification_id": "ntf_test01",
		"data":            data,
	}
	body, _ := json.Marshal(env)
	return body, paddleSign(paddleTestSecret, body)
}

func paddleBody(eventType, eventID string, data map[string]any) ([]byte, string) {
	return paddleBodyAt(eventType, eventID, time.Now(), data)
}

// paddleSubData builds a subscription entity; periods default to a live
// window around now so merge outcomes reflect "currently granting" states.
func paddleSubData(status string, extra map[string]any) map[string]any {
	data := map[string]any{
		"id":            paddleTestSubID,
		"status":        status,
		"customer_id":   paddleTestCustomerID,
		"currency_code": "USD",
		"billing_cycle": map[string]any{"interval": "month", "frequency": 1},
		"current_billing_period": map[string]any{
			"starts_at": time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339),
			"ends_at":   time.Now().Add(30 * 24 * time.Hour).UTC().Format(time.RFC3339),
		},
		"items": []any{
			map[string]any{
				"status":    "active",
				"quantity":  1,
				"recurring": true,
				"price":     map[string]any{"id": "pri_01gsz8x8sawmvhz1pv30nge1ke", "product_id": "pro_01gsz4t5hdjse780zja8vvr7jg"},
			},
		},
	}
	for k, v := range extra {
		data[k] = v
	}
	return data
}

func paddleAdjustmentData(action, status, adjType string, extra map[string]any) map[string]any {
	data := map[string]any{
		"id":              "adj_01h_test",
		"action":          action,
		"status":          status,
		"type":            adjType,
		"transaction_id":  paddleTestTxnID,
		"customer_id":     paddleTestCustomerID,
		"subscription_id": paddleTestSubID,
		"reason":          "Requested by customer",
	}
	for k, v := range extra {
		data[k] = v
	}
	return data
}

// linkedMock returns a mock where the paddle customer id is already linked to
// the test user (as a completed checkout + subscription event would leave it).
func linkedMock() *mockBilling {
	m := newMockBilling()
	m.plans = paddleTestPlans
	m.byCustomer[paddleTestCustomerID] = paddleTestUserID
	return m
}

// seedPaddleState installs a live paddle provider state for the test user.
func seedPaddleState(m *mockBilling, status, subID string, periodEnd time.Time, lastEventAt time.Time) {
	s := db.SubscriptionProviderState{
		UserID:   paddleTestUserID,
		Provider: "paddle",
		Status:   status,
		CurrentPeriodStart: pgtype.Timestamptz{
			Time: periodEnd.Add(-30 * 24 * time.Hour), Valid: true,
		},
		CurrentPeriodEnd:       pgtype.Timestamptz{Time: periodEnd, Valid: true},
		ProviderCustomerID:     &paddleTestCustomerID,
		ProviderSubscriptionID: &subID,
	}
	if !lastEventAt.IsZero() {
		s.LastEventAt = pgtype.Timestamptz{Time: lastEventAt, Valid: true}
	}
	m.seedState(s)
}

// ─── Envelope-level tests ───────────────────────────────────────────────────

func TestHandlePaddleWebhook_NotConfigured(t *testing.T) {
	l := paddleTestLogic(newMockBilling())
	l.svcCtx.Config.Billing.Paddle.Enabled = false

	_, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: []byte(`{}`), Signature: "x"})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestHandlePaddleWebhook_InvalidSignature(t *testing.T) {
	l := paddleTestLogic(newMockBilling())

	body := []byte(`{"event_id":"evt_x","event_type":"subscription.updated","data":{}}`)
	_, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{
		RawBody:   body,
		Signature: paddleSign("wrong_secret", body),
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHandlePaddleWebhook_MalformedBody(t *testing.T) {
	l := paddleTestLogic(newMockBilling())

	body := []byte(`{not json`)
	_, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{
		RawBody:   body,
		Signature: paddleSign(paddleTestSecret, body),
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHandlePaddleWebhook_DuplicateEvent(t *testing.T) {
	m := newMockBilling()
	m.allProcessed = true
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.updated", "evt_dup01", paddleSubData("active", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls)
	assert.Empty(t, m.paddleMarkCalls)
}

func TestHandlePaddleWebhook_UnknownEventType(t *testing.T) {
	m := linkedMock()
	l := paddleTestLogic(m)

	body, sig := paddleBody("product.updated", "evt_unk01", map[string]any{"id": "pro_x"})
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls)
	assert.Equal(t, []string{"evt_unk01"}, m.paddleMarkCalls)
}

// ─── subscription.* events → provider state + merged projection ─────────────

func TestHandlePaddleWebhook_SubscriptionUpdated_Active(t *testing.T) {
	m := linkedMock()
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.updated", "evt_sub01", paddleSubData("active", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	// Provider state written for 'paddle' only.
	require.Len(t, m.upsertCalls, 1)
	p := m.upsertCalls[0]
	assert.Equal(t, paddleTestUserID, p.UserID)
	assert.Equal(t, "paddle", p.Provider)
	assert.Equal(t, "active", p.Status)
	require.NotNil(t, p.BillingInterval)
	assert.Equal(t, "monthly", *p.BillingInterval)
	assert.False(t, p.CancelAtPeriodEnd)
	assert.True(t, p.CurrentPeriodEnd.Valid)
	assert.Equal(t, paddleTestCustomerID, *p.ProviderCustomerID)
	assert.Equal(t, paddleTestSubID, *p.ProviderSubscriptionID)

	// Merged projection grants pro.
	merged := m.lastMerged(paddleTestUserID)
	assert.Equal(t, "active", merged.Status)
	assert.Equal(t, paddleProPlanID, merged.PlanID)
	assert.Equal(t, paddleTestCustomerID, *merged.PaddleCustomerID)

	assert.Equal(t, []string{"evt_sub01"}, m.paddleMarkCalls)
	assert.Equal(t, 1, m.byCustomerCalls)
}

func TestHandlePaddleWebhook_SubscriptionUpdated_ScheduledCancel(t *testing.T) {
	m := linkedMock()
	l := paddleTestLogic(m)

	effective := time.Now().Add(14 * 24 * time.Hour).UTC().Format(time.RFC3339)
	body, sig := paddleBody("subscription.updated", "evt_sub02", paddleSubData("active", map[string]any{
		"scheduled_change": map[string]any{"action": "cancel", "effective_at": effective, "resume_at": nil},
	}))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "active", m.upsertCalls[0].Status)
	assert.True(t, m.upsertCalls[0].CancelAtPeriodEnd)

	merged := m.lastMerged(paddleTestUserID)
	assert.Equal(t, "active", merged.Status)
	assert.Equal(t, paddleProPlanID, merged.PlanID)
	assert.True(t, merged.CancelAtPeriodEnd)
}

func TestHandlePaddleWebhook_SubscriptionUpdated_ScheduledPause(t *testing.T) {
	m := linkedMock()
	l := paddleTestLogic(m)

	effective := time.Now().Add(14 * 24 * time.Hour).UTC().Format(time.RFC3339)
	body, sig := paddleBody("subscription.updated", "evt_sub_pause", paddleSubData("active", map[string]any{
		"scheduled_change": map[string]any{"action": "pause", "effective_at": effective, "resume_at": nil},
	}))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "active", m.upsertCalls[0].Status)
	assert.True(t, m.upsertCalls[0].CancelAtPeriodEnd)
}

func TestHandlePaddleWebhook_SubscriptionCanceled(t *testing.T) {
	m := linkedMock()
	seedPaddleState(m, "active", paddleTestSubID, time.Now().Add(10*24*time.Hour), time.Time{})
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.canceled", "evt_sub03", paddleSubData("canceled", map[string]any{
		"scheduled_change": map[string]any{"action": "cancel", "effective_at": time.Now().UTC().Format(time.RFC3339), "resume_at": nil},
	}))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "canceled", m.upsertCalls[0].Status)

	merged := m.lastMerged(paddleTestUserID)
	assert.Equal(t, "canceled", merged.Status)
	assert.Equal(t, paddleFreePlanID, merged.PlanID)
}

func TestHandlePaddleWebhook_SubscriptionEvent_NoUser(t *testing.T) {
	// No checkout record and no customer link → permanent failure: the event
	// is marked processed so Paddle stops retrying.
	m := newMockBilling()
	m.plans = paddleTestPlans
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.created", "evt_sub05", paddleSubData("active", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls)
	assert.Equal(t, []string{"evt_sub05"}, m.paddleMarkCalls)
}

func TestHandlePaddleWebhook_SubscriptionCreated_UntrustedCustomData(t *testing.T) {
	// B4: custom_data.user_id on a subscription event is never consulted —
	// resolution is via the stored customer link only. An event that only
	// carries a (possibly spoofed) custom_data user id is unprocessable.
	m := newMockBilling()
	m.plans = paddleTestPlans
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.created", "evt_sub_spoof", paddleSubData("active", map[string]any{
		"custom_data": map[string]any{"user_id": paddleTestUserID.String()},
	}))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls)
	assert.Equal(t, []string{"evt_sub_spoof"}, m.paddleMarkCalls)
}

func TestHandlePaddleWebhook_StaleSubscriptionIDIgnored(t *testing.T) {
	// A different, live subscription is tracked — events for a superseded
	// sub_ id must not clobber it.
	m := linkedMock()
	oldSubID := "sub_01h9999999999999999999999a"
	seedPaddleState(m, "active", oldSubID, time.Now().Add(20*24*time.Hour), time.Time{})
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.updated", "evt_sub06", paddleSubData("canceled", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls)
	assert.Equal(t, "active", m.paddleState(paddleTestUserID).Status)
	assert.Equal(t, []string{"evt_sub06"}, m.paddleMarkCalls)
}

func TestHandlePaddleWebhook_OutOfOrderEventIgnored(t *testing.T) {
	// B2: an event older than the provider watermark must not roll state back.
	m := linkedMock()
	lastEvent := time.Now()
	seedPaddleState(m, "active", paddleTestSubID, time.Now().Add(20*24*time.Hour), lastEvent)
	l := paddleTestLogic(m)

	// A delayed cancellation that predates the last applied event.
	body, sig := paddleBodyAt("subscription.canceled", "evt_stale01",
		lastEvent.Add(-time.Hour), paddleSubData("canceled", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls, "stale event must not write provider state")
	assert.Equal(t, "active", m.paddleState(paddleTestUserID).Status)
	assert.Equal(t, []string{"evt_stale01"}, m.paddleMarkCalls)
}

func TestHandlePaddleWebhook_NewerEventApplies(t *testing.T) {
	m := linkedMock()
	lastEvent := time.Now().Add(-time.Hour)
	seedPaddleState(m, "canceled", paddleTestSubID, time.Now().Add(-time.Hour), lastEvent)
	l := paddleTestLogic(m)

	// A renewal that happened after the canceled event was applied.
	body, sig := paddleBodyAt("subscription.updated", "evt_new01",
		time.Now(), paddleSubData("active", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "active", m.upsertCalls[0].Status)
}

func TestHandlePaddleWebhook_PastDueKeepsPro(t *testing.T) {
	m := linkedMock()
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.past_due", "evt_sub07", paddleSubData("past_due", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "past_due", m.upsertCalls[0].Status)
	merged := m.lastMerged(paddleTestUserID)
	assert.Equal(t, "past_due", merged.Status)
	assert.Equal(t, paddleProPlanID, merged.PlanID)
}

func TestHandlePaddleWebhook_Paused(t *testing.T) {
	m := linkedMock()
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.paused", "evt_sub08", paddleSubData("paused", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "paused", m.upsertCalls[0].Status)
	// Paused keeps the pro plan (resumable) but does not grant access — the
	// merged status is 'paused', which is not a granting status.
	merged := m.lastMerged(paddleTestUserID)
	assert.Equal(t, "paused", merged.Status)
	assert.Equal(t, paddleProPlanID, merged.PlanID)
}

func TestHandlePaddleWebhook_YearlyInterval(t *testing.T) {
	m := linkedMock()
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.updated", "evt_sub09", paddleSubData("active", map[string]any{
		"billing_cycle": map[string]any{"interval": "year", "frequency": 1},
	}))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	require.Len(t, m.upsertCalls, 1)
	require.NotNil(t, m.upsertCalls[0].BillingInterval)
	assert.Equal(t, "annual", *m.upsertCalls[0].BillingInterval)
}

// ─── B1: cross-provider merge ───────────────────────────────────────────────

func TestHandlePaddleWebhook_PaddleCancelDoesNotKillActiveRevenueCat(t *testing.T) {
	// The user has an active RevenueCat subscription; a Paddle cancellation
	// must only update the paddle provider row — the merged row stays pro.
	m := linkedMock()
	seedPaddleState(m, "active", paddleTestSubID, time.Now().Add(10*24*time.Hour), time.Time{})
	m.seedState(db.SubscriptionProviderState{
		UserID:             paddleTestUserID,
		Provider:           "revenuecat",
		Status:             "active",
		CurrentPeriodEnd:   pgtype.Timestamptz{Time: time.Now().Add(40 * 24 * time.Hour), Valid: true},
		CurrentPeriodStart: pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true},
	})
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.canceled", "evt_b1_01", paddleSubData("canceled", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	// Paddle provider state recorded the cancel...
	assert.Equal(t, "canceled", m.paddleState(paddleTestUserID).Status)
	// ...but the merged projection still grants pro via RevenueCat.
	merged := m.lastMerged(paddleTestUserID)
	assert.Equal(t, "active", merged.Status)
	assert.Equal(t, paddleProPlanID, merged.PlanID)
}

// ─── transaction.completed ──────────────────────────────────────────────────

func paddleTxnData(extra map[string]any) map[string]any {
	data := map[string]any{
		"id":              paddleTestTxnID,
		"status":          "completed",
		"customer_id":     paddleTestCustomerID,
		"subscription_id": paddleTestSubID,
		"currency_code":   "USD",
		"collection_mode": "automatic",
		"items":           []any{map[string]any{"price_id": "pri_01gsz8x8sawmvhz1pv30nge1ke", "quantity": 1}},
	}
	for k, v := range extra {
		data[k] = v
	}
	return data
}

func TestHandlePaddleWebhook_TransactionCompleted_ViaCheckoutRecord(t *testing.T) {
	m := newMockBilling()
	m.plans = paddleTestPlans
	m.checkouts[paddleTestTxnID] = paddleTestUserID // created by our backend
	l := paddleTestLogic(m)

	body, sig := paddleBody("transaction.completed", "evt_txn01", paddleTxnData(nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	// Audit event recorded for backend checkouts.
	require.Len(t, m.upgradeEvents, 1)
	assert.Equal(t, "checkout_completed", m.upgradeEvents[0].EventType)
	assert.Equal(t, "paddle_webhook", m.upgradeEvents[0].Surface)
	assert.Equal(t, paddleTestUserID, m.upgradeEvents[0].UserID)

	// IDs linked on the provider state.
	require.Len(t, m.linkCalls, 1)
	assert.Equal(t, paddleTestCustomerID, *m.linkCalls[0].customerID)
	assert.Equal(t, paddleTestSubID, *m.linkCalls[0].subscriptionID)

	// Provider state exists now (status 'free' — subscription.* sets the rest)
	// and the merged row carries the customer link for future resolutions.
	merged := m.lastMerged(paddleTestUserID)
	require.NotNil(t, merged.PaddleCustomerID)
	assert.Equal(t, paddleTestCustomerID, *merged.PaddleCustomerID)
	assert.Equal(t, []string{"evt_txn01"}, m.paddleMarkCalls)
}

func TestHandlePaddleWebhook_TransactionCompleted_SpoofedCustomData(t *testing.T) {
	// B4: the backend-recorded checkout owner wins over client-provided
	// custom_data — a spoofed user_id cannot redirect the purchase.
	m := newMockBilling()
	m.plans = paddleTestPlans
	m.checkouts[paddleTestTxnID] = paddleTestUserID
	l := paddleTestLogic(m)

	body, sig := paddleBody("transaction.completed", "evt_txn_spoof", paddleTxnData(map[string]any{
		"custom_data": map[string]any{"user_id": paddleTestOtherUser.String()},
	}))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.linkCalls, 1)
	assert.Equal(t, paddleTestUserID, m.linkCalls[0].userID, "must bind to the recorded checkout creator")
	assert.Equal(t, paddleTestUserID, m.upgradeEvents[0].UserID)
}

func TestHandlePaddleWebhook_TransactionCompleted_UntrustedCustomData(t *testing.T) {
	// B4: a transaction we did NOT create carries attacker-controlled
	// custom_data — it is ignored. With no customer link either, the event is
	// permanently unprocessable.
	m := newMockBilling()
	m.plans = paddleTestPlans
	l := paddleTestLogic(m)

	body, sig := paddleBody("transaction.completed", "evt_txn_untrusted", paddleTxnData(map[string]any{
		"id":          "txn_client_side_01",
		"custom_data": map[string]any{"user_id": paddleTestUserID.String()},
	}))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.linkCalls)
	assert.Empty(t, m.upgradeEvents)
	assert.Equal(t, []string{"evt_txn_untrusted"}, m.paddleMarkCalls)
}

func TestHandlePaddleWebhook_TransactionCompleted_RenewalViaCustomerLink(t *testing.T) {
	// Renewal transactions aren't backend checkouts — they resolve via the
	// stored customer link and don't record a checkout_completed event.
	m := linkedMock()
	l := paddleTestLogic(m)

	body, sig := paddleBody("transaction.completed", "evt_txn_renew", paddleTxnData(map[string]any{
		"id": "txn_renewal_01",
	}))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	assert.Empty(t, m.upgradeEvents, "renewals are not funnel events")
	require.Len(t, m.linkCalls, 1)
	assert.Equal(t, paddleTestUserID, m.linkCalls[0].userID)
}

// ─── adjustment.* events (B5) ───────────────────────────────────────────────

func TestHandlePaddleWebhook_FullRefundRevokes(t *testing.T) {
	m := linkedMock()
	seedPaddleState(m, "active", paddleTestSubID, time.Now().Add(20*24*time.Hour), time.Time{})
	l := paddleTestLogic(m)

	body, sig := paddleBody("adjustment.created", "evt_adj_full", paddleAdjustmentData("refund", "approved", "full", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	// Paddle access revoked.
	assert.Equal(t, "expired", m.paddleState(paddleTestUserID).Status)
	merged := m.lastMerged(paddleTestUserID)
	assert.Equal(t, "expired", merged.Status)
	assert.Equal(t, paddleFreePlanID, merged.PlanID)

	// Audit event recorded.
	require.Len(t, m.upgradeEvents, 1)
	assert.Equal(t, "subscription_refunded", m.upgradeEvents[0].EventType)
}

func TestHandlePaddleWebhook_ChargebackRevokes(t *testing.T) {
	m := linkedMock()
	seedPaddleState(m, "active", paddleTestSubID, time.Now().Add(20*24*time.Hour), time.Time{})
	l := paddleTestLogic(m)

	body, sig := paddleBody("adjustment.created", "evt_adj_cb", paddleAdjustmentData("chargeback", "approved", "", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	assert.Equal(t, "expired", m.paddleState(paddleTestUserID).Status)
	require.Len(t, m.upgradeEvents, 1)
	assert.Equal(t, "subscription_chargeback", m.upgradeEvents[0].EventType)
}

func TestHandlePaddleWebhook_RefundDoesNotKillActiveRevenueCat(t *testing.T) {
	// B1+B5: a web refund revokes only the paddle state — a live mobile
	// subscription still wins the merge.
	m := linkedMock()
	seedPaddleState(m, "active", paddleTestSubID, time.Now().Add(20*24*time.Hour), time.Time{})
	m.seedState(db.SubscriptionProviderState{
		UserID:             paddleTestUserID,
		Provider:           "revenuecat",
		Status:             "active",
		CurrentPeriodEnd:   pgtype.Timestamptz{Time: time.Now().Add(40 * 24 * time.Hour), Valid: true},
		CurrentPeriodStart: pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true},
	})
	l := paddleTestLogic(m)

	body, sig := paddleBody("adjustment.created", "evt_adj_rc", paddleAdjustmentData("refund", "approved", "full", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	assert.Equal(t, "expired", m.paddleState(paddleTestUserID).Status)
	merged := m.lastMerged(paddleTestUserID)
	assert.Equal(t, "active", merged.Status)
	assert.Equal(t, paddleProPlanID, merged.PlanID)
}

func TestHandlePaddleWebhook_PartialRefundKeepsAccess(t *testing.T) {
	m := linkedMock()
	seedPaddleState(m, "active", paddleTestSubID, time.Now().Add(20*24*time.Hour), time.Time{})
	l := paddleTestLogic(m)

	body, sig := paddleBody("adjustment.created", "evt_adj_partial", paddleAdjustmentData("refund", "approved", "partial", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	assert.Equal(t, "active", m.paddleState(paddleTestUserID).Status)
	assert.Empty(t, m.upgradeEvents)
}

func TestHandlePaddleWebhook_PendingRefundDoesNothing(t *testing.T) {
	m := linkedMock()
	seedPaddleState(m, "active", paddleTestSubID, time.Now().Add(20*24*time.Hour), time.Time{})
	l := paddleTestLogic(m)

	body, sig := paddleBody("adjustment.created", "evt_adj_pending", paddleAdjustmentData("refund", "pending_approval", "full", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Equal(t, "active", m.paddleState(paddleTestUserID).Status)
	assert.Empty(t, m.upgradeEvents)
}

func TestHandlePaddleWebhook_ChargebackReversalResyncs(t *testing.T) {
	m := linkedMock()
	seedPaddleState(m, "expired", paddleTestSubID, time.Now(), time.Time{})
	l := paddleTestLogic(m)

	// No Paddle API client in tests → post-commit resync is a no-op; the
	// restore audit event + processed marking is what we assert.
	body, sig := paddleBody("adjustment.updated", "evt_adj_rev", paddleAdjustmentData("chargeback_reverse", "reversed", "", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upgradeEvents, 1)
	assert.Equal(t, "subscription_restored", m.upgradeEvents[0].EventType)
	assert.Equal(t, []string{"evt_adj_rev"}, m.paddleMarkCalls)
}

func TestHandlePaddleWebhook_AdjustmentNoUser(t *testing.T) {
	m := newMockBilling()
	m.plans = paddleTestPlans
	l := paddleTestLogic(m)

	body, sig := paddleBody("adjustment.created", "evt_adj_nouser", paddleAdjustmentData("chargeback", "approved", "", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upgradeEvents)
	assert.Equal(t, []string{"evt_adj_nouser"}, m.paddleMarkCalls)
}

func TestHandlePaddleWebhook_MarkProcessedError_Retries(t *testing.T) {
	m := linkedMock()
	m.markProcessedErr = errors.New("db down")
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.updated", "evt_sub10", paddleSubData("active", nil))
	_, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.Error(t, err) // tx rolled back → Paddle retries
}

func TestMapPaddleStatus(t *testing.T) {
	assert.Equal(t, "active", mapPaddleStatus("active"))
	assert.Equal(t, "trialing", mapPaddleStatus("trialing"))
	assert.Equal(t, "past_due", mapPaddleStatus("past_due"))
	assert.Equal(t, "paused", mapPaddleStatus("paused"))
	assert.Equal(t, "canceled", mapPaddleStatus("canceled"))
	assert.Equal(t, "expired", mapPaddleStatus("something_new"))
}
