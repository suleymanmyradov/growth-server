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
	"github.com/jackc/pgx/v5"
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

// paddleMockBilling is a configurable mock for the Paddle webhook tests.
type paddleMockBilling struct {
	getOrCreateSub   db.GetUserSubscriptionRow
	getOrCreateErr   error
	getSub           db.GetUserSubscriptionRow
	getSubErr        error
	getPlanByCode    map[string]db.Plan
	byCustomerRow    db.GetUserSubscriptionByPaddleCustomerIDRow
	byCustomerErr    error
	isProcessed      bool
	isProcessedErr   error
	markProcessedErr error
	upsertErr        error
	upgradeEventErr  error

	upsertCalls       []db.UpsertUserSubscriptionPaddleParams
	upgradeEventCalls []db.CreateUpgradeEventParams
	markCalls         []string
	byCustomerCalls   int
}

func (m *paddleMockBilling) ListActivePlans(ctx context.Context) ([]db.Plan, error) {
	panic("not used")
}
func (m *paddleMockBilling) GetPlanByCode(_ context.Context, code string) (db.Plan, error) {
	if p, ok := m.getPlanByCode[code]; ok {
		return p, nil
	}
	return db.Plan{}, errors.New("plan not found")
}
func (m *paddleMockBilling) GetUserSubscription(_ context.Context, _ uuid.UUID) (db.GetUserSubscriptionRow, error) {
	if m.getSubErr != nil {
		return db.GetUserSubscriptionRow{}, m.getSubErr
	}
	return m.getSub, nil
}
func (m *paddleMockBilling) GetOrCreateUserSubscription(_ context.Context, userID uuid.UUID) (db.GetUserSubscriptionRow, error) {
	if m.getOrCreateErr != nil {
		return db.GetUserSubscriptionRow{}, m.getOrCreateErr
	}
	return m.getOrCreateSub, nil
}
func (m *paddleMockBilling) CreateDefaultFreeSubscription(ctx context.Context, userID uuid.UUID) (db.Subscription, error) {
	panic("not used")
}
func (m *paddleMockBilling) UpsertUserSubscription(ctx context.Context, _ db.UpsertUserSubscriptionParams) (db.Subscription, error) {
	panic("not used")
}
func (m *paddleMockBilling) CreateUpgradeEvent(_ context.Context, params db.CreateUpgradeEventParams) (db.CreateUpgradeEventRow, error) {
	m.upgradeEventCalls = append(m.upgradeEventCalls, params)
	if m.upgradeEventErr != nil {
		return db.CreateUpgradeEventRow{}, m.upgradeEventErr
	}
	return db.CreateUpgradeEventRow{}, nil
}
func (m *paddleMockBilling) ComputeEntitlements(ctx context.Context, _ db.GetUserSubscriptionRow, _ uuid.UUID) (*repository.EntitlementsResult, error) {
	panic("not used")
}

func (m *paddleMockBilling) ListSubscriptionStatuses(ctx context.Context) ([]db.ListSubscriptionStatusesRow, error) {
	panic("not used")
}
func (m *paddleMockBilling) GetUserSubscriptionByUserID(ctx context.Context, _ uuid.UUID) (db.GetUserSubscriptionByUserIDRow, error) {
	panic("not used")
}
func (m *paddleMockBilling) SetRevenueCatCustomerID(ctx context.Context, _ uuid.UUID, _ *string) error {
	panic("not used")
}
func (m *paddleMockBilling) IsRevenueCatEventProcessed(ctx context.Context, _ string) (bool, error) {
	panic("not used")
}
func (m *paddleMockBilling) MarkRevenueCatEventProcessed(ctx context.Context, _ string) error {
	panic("not used")
}
func (m *paddleMockBilling) GetUserSubscriptionByPaddleCustomerID(_ context.Context, _ *string) (db.GetUserSubscriptionByPaddleCustomerIDRow, error) {
	m.byCustomerCalls++
	if m.byCustomerErr != nil {
		return db.GetUserSubscriptionByPaddleCustomerIDRow{}, m.byCustomerErr
	}
	return m.byCustomerRow, nil
}
func (m *paddleMockBilling) SetPaddleCustomerID(ctx context.Context, _ uuid.UUID, _ *string) error {
	panic("not used")
}
func (m *paddleMockBilling) UpsertUserSubscriptionPaddle(_ context.Context, params db.UpsertUserSubscriptionPaddleParams) (db.Subscription, error) {
	m.upsertCalls = append(m.upsertCalls, params)
	if m.upsertErr != nil {
		return db.Subscription{}, m.upsertErr
	}
	return db.Subscription{}, nil
}
func (m *paddleMockBilling) IsPaddleEventProcessed(_ context.Context, _ string) (bool, error) {
	if m.isProcessedErr != nil {
		return false, m.isProcessedErr
	}
	return m.isProcessed, nil
}
func (m *paddleMockBilling) MarkPaddleEventProcessed(_ context.Context, eventID string) error {
	m.markCalls = append(m.markCalls, eventID)
	return m.markProcessedErr
}

// paddleTestLogic builds the logic with a configured webhook secret and the
// mock repo / no-op tx runner.
func paddleTestLogic(m *paddleMockBilling) *HandlePaddleWebhookLogic {
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

// paddleBody builds a signed event envelope body + signature header.
func paddleBody(eventType, eventID string, data map[string]any) ([]byte, string) {
	env := map[string]any{
		"event_id":        eventID,
		"event_type":      eventType,
		"occurred_at":     time.Now().UTC().Format(time.RFC3339),
		"notification_id": "ntf_test01",
		"data":            data,
	}
	body, _ := json.Marshal(env)
	return body, paddleSign(paddleTestSecret, body)
}

var (
	paddleTestUserID = uuid.MustParse("0191fa87-6ed1-7022-9999-0123456789ae")
	paddleProPlanID  = uuid.MustParse("0191fa87-0000-7000-8000-0000000000a1")
	paddleFreePlanID = uuid.MustParse("0191fa87-0000-7000-8000-0000000000f1")
	paddleTestPlans  = map[string]db.Plan{
		"pro":  {ID: paddleProPlanID, Code: "pro"},
		"free": {ID: paddleFreePlanID, Code: "free"},
	}
	paddleTestCustomerID = "ctm_01h8441jn5pcwrfhwh78jqt8hk"
	paddleTestSubID      = "sub_01h849vje9skebr9p3bx2g0mxn"
)

func paddleSubData(status string, extra map[string]any) map[string]any {
	data := map[string]any{
		"id":            paddleTestSubID,
		"status":        status,
		"customer_id":   paddleTestCustomerID,
		"currency_code": "USD",
		"billing_cycle": map[string]any{"interval": "month", "frequency": 1},
		"current_billing_period": map[string]any{
			"starts_at": "2024-04-11T15:57:24.800Z",
			"ends_at":   "2024-05-11T15:57:24.800Z",
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

func TestHandlePaddleWebhook_NotConfigured(t *testing.T) {
	m := &paddleMockBilling{}
	l := paddleTestLogic(m)
	l.svcCtx.Config.Billing.Paddle.Enabled = false

	_, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: []byte(`{}`), Signature: "x"})
	require.Error(t, err)
	assert.Equal(t, codes.FailedPrecondition, status.Code(err))
}

func TestHandlePaddleWebhook_InvalidSignature(t *testing.T) {
	m := &paddleMockBilling{}
	l := paddleTestLogic(m)

	body := []byte(`{"event_id":"evt_x","event_type":"subscription.updated","data":{}}`)
	_, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{
		RawBody:   body,
		Signature: paddleSign("wrong_secret", body),
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHandlePaddleWebhook_MalformedBody(t *testing.T) {
	m := &paddleMockBilling{}
	l := paddleTestLogic(m)

	body := []byte(`{not json`)
	_, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{
		RawBody:   body,
		Signature: paddleSign(paddleTestSecret, body),
	})
	require.Error(t, err)
	assert.Equal(t, codes.InvalidArgument, status.Code(err))
}

func TestHandlePaddleWebhook_DuplicateEvent(t *testing.T) {
	m := &paddleMockBilling{isProcessed: true}
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.updated", "evt_dup01", paddleSubData("active", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls)
	assert.Empty(t, m.markCalls)
}

func TestHandlePaddleWebhook_UnknownEventType(t *testing.T) {
	m := &paddleMockBilling{getPlanByCode: paddleTestPlans}
	l := paddleTestLogic(m)

	body, sig := paddleBody("product.updated", "evt_unk01", map[string]any{"id": "pro_x"})
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls)
	assert.Equal(t, []string{"evt_unk01"}, m.markCalls)
}

func TestHandlePaddleWebhook_SubscriptionUpdated_Active(t *testing.T) {
	m := &paddleMockBilling{
		getPlanByCode: paddleTestPlans,
		byCustomerRow: db.GetUserSubscriptionByPaddleCustomerIDRow{UserID: paddleTestUserID, Status: "active"},
		getOrCreateSub: db.GetUserSubscriptionRow{
			UserID: paddleTestUserID, PlanID: paddleFreePlanID, Status: "active",
		},
	}
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.updated", "evt_sub01", paddleSubData("active", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	p := m.upsertCalls[0]
	assert.Equal(t, paddleTestUserID, p.UserID)
	assert.Equal(t, paddleProPlanID, p.PlanID)
	assert.Equal(t, "active", p.Status)
	require.NotNil(t, p.BillingInterval)
	assert.Equal(t, "monthly", *p.BillingInterval)
	assert.False(t, p.CancelAtPeriodEnd)
	assert.True(t, p.CurrentPeriodEnd.Valid)
	assert.Equal(t, paddleTestCustomerID, *p.PaddleCustomerID)
	assert.Equal(t, paddleTestSubID, *p.PaddleSubscriptionID)
	assert.Equal(t, []string{"evt_sub01"}, m.markCalls)
	// No custom_data on subscription events — resolved via customer id link.
	assert.Equal(t, 1, m.byCustomerCalls)
}

func TestHandlePaddleWebhook_SubscriptionUpdated_ScheduledCancel(t *testing.T) {
	m := &paddleMockBilling{
		getPlanByCode: paddleTestPlans,
		byCustomerRow: db.GetUserSubscriptionByPaddleCustomerIDRow{UserID: paddleTestUserID, Status: "active"},
		getOrCreateSub: db.GetUserSubscriptionRow{
			UserID: paddleTestUserID, PlanID: paddleProPlanID, Status: "active",
			PaddleSubscriptionID: &paddleTestSubID,
		},
	}
	l := paddleTestLogic(m)

	effective := time.Now().Add(14 * 24 * time.Hour).UTC().Format(time.RFC3339)
	body, sig := paddleBody("subscription.updated", "evt_sub02", paddleSubData("active", map[string]any{
		"scheduled_change": map[string]any{"action": "cancel", "effective_at": effective, "resume_at": nil},
	}))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	p := m.upsertCalls[0]
	// Still active with pro access until the period ends; the flag carries
	// the pending cancellation.
	assert.Equal(t, "active", p.Status)
	assert.Equal(t, paddleProPlanID, p.PlanID)
	assert.True(t, p.CancelAtPeriodEnd)
}

func TestHandlePaddleWebhook_SubscriptionUpdated_ScheduledPause(t *testing.T) {
	m := &paddleMockBilling{
		getPlanByCode: paddleTestPlans,
		byCustomerRow: db.GetUserSubscriptionByPaddleCustomerIDRow{UserID: paddleTestUserID, Status: "active"},
		getOrCreateSub: db.GetUserSubscriptionRow{
			UserID: paddleTestUserID, PlanID: paddleProPlanID, Status: "active",
			PaddleSubscriptionID: &paddleTestSubID,
		},
	}
	l := paddleTestLogic(m)

	effective := time.Now().Add(14 * 24 * time.Hour).UTC().Format(time.RFC3339)
	body, sig := paddleBody("subscription.updated", "evt_sub_pause", paddleSubData("active", map[string]any{
		"scheduled_change": map[string]any{"action": "pause", "effective_at": effective, "resume_at": nil},
	}))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	p := m.upsertCalls[0]
	// A scheduled pause ends paid access at effective_at, same as a cancel —
	// the flag carries the pending change until the 'paused' event lands.
	assert.Equal(t, "active", p.Status)
	assert.True(t, p.CancelAtPeriodEnd)
}

func TestHandlePaddleWebhook_SubscriptionCanceled(t *testing.T) {
	m := &paddleMockBilling{
		getPlanByCode: paddleTestPlans,
		byCustomerRow: db.GetUserSubscriptionByPaddleCustomerIDRow{UserID: paddleTestUserID, Status: "canceled"},
		getOrCreateSub: db.GetUserSubscriptionRow{
			UserID: paddleTestUserID, PlanID: paddleProPlanID, Status: "active",
			PaddleSubscriptionID: &paddleTestSubID,
		},
	}
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.canceled", "evt_sub03", paddleSubData("canceled", map[string]any{
		"scheduled_change": map[string]any{"action": "cancel", "effective_at": "2024-05-11T15:57:24.800Z", "resume_at": nil},
	}))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	p := m.upsertCalls[0]
	assert.Equal(t, "canceled", p.Status)
	assert.Equal(t, paddleFreePlanID, p.PlanID)
}

func TestHandlePaddleWebhook_SubscriptionCreated_ViaCustomData(t *testing.T) {
	m := &paddleMockBilling{
		getPlanByCode: paddleTestPlans,
		getOrCreateSub: db.GetUserSubscriptionRow{
			UserID: paddleTestUserID, PlanID: paddleFreePlanID, Status: "active",
		},
	}
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.created", "evt_sub04", paddleSubData("active", map[string]any{
		"custom_data": map[string]any{"user_id": paddleTestUserID.String()},
	}))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, paddleTestUserID, m.upsertCalls[0].UserID)
	// Resolved straight from custom_data — no customer-id lookup needed.
	assert.Equal(t, 0, m.byCustomerCalls)
}

func TestHandlePaddleWebhook_SubscriptionEvent_NoUser(t *testing.T) {
	// No custom_data and no customer link → permanent failure: event is
	// marked processed so Paddle stops retrying.
	m := &paddleMockBilling{
		getPlanByCode: paddleTestPlans,
		byCustomerErr: pgx.ErrNoRows,
	}
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.created", "evt_sub05", paddleSubData("active", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls)
	assert.Equal(t, []string{"evt_sub05"}, m.markCalls)
}

func TestHandlePaddleWebhook_StaleSubscriptionIgnored(t *testing.T) {
	// The DB already tracks a different, live paddle subscription — an event
	// for an old sub id must not clobber it.
	otherSubID := "sub_01h9999999999999999999999a"
	m := &paddleMockBilling{
		getPlanByCode: paddleTestPlans,
		byCustomerRow: db.GetUserSubscriptionByPaddleCustomerIDRow{UserID: paddleTestUserID, Status: "active"},
		getOrCreateSub: db.GetUserSubscriptionRow{
			UserID: paddleTestUserID, PlanID: paddleProPlanID, Status: "active",
			PaddleSubscriptionID: &otherSubID,
		},
	}
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.updated", "evt_sub06", paddleSubData("canceled", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls)
	assert.Equal(t, []string{"evt_sub06"}, m.markCalls)
}

func TestHandlePaddleWebhook_PastDueKeepsPro(t *testing.T) {
	m := &paddleMockBilling{
		getPlanByCode: paddleTestPlans,
		byCustomerRow: db.GetUserSubscriptionByPaddleCustomerIDRow{UserID: paddleTestUserID, Status: "past_due"},
		getOrCreateSub: db.GetUserSubscriptionRow{
			UserID: paddleTestUserID, PlanID: paddleProPlanID, Status: "active",
			PaddleSubscriptionID: &paddleTestSubID,
		},
	}
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.past_due", "evt_sub07", paddleSubData("past_due", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	p := m.upsertCalls[0]
	assert.Equal(t, "past_due", p.Status)
	assert.Equal(t, paddleProPlanID, p.PlanID)
}

func TestHandlePaddleWebhook_Paused(t *testing.T) {
	m := &paddleMockBilling{
		getPlanByCode: paddleTestPlans,
		byCustomerRow: db.GetUserSubscriptionByPaddleCustomerIDRow{UserID: paddleTestUserID, Status: "paused"},
		getOrCreateSub: db.GetUserSubscriptionRow{
			UserID: paddleTestUserID, PlanID: paddleProPlanID, Status: "active",
			PaddleSubscriptionID: &paddleTestSubID,
		},
	}
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.paused", "evt_sub08", paddleSubData("paused", nil))
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "paused", m.upsertCalls[0].Status)
	assert.Equal(t, paddleProPlanID, m.upsertCalls[0].PlanID)
}

func TestHandlePaddleWebhook_YearlyInterval(t *testing.T) {
	m := &paddleMockBilling{
		getPlanByCode: paddleTestPlans,
		byCustomerRow: db.GetUserSubscriptionByPaddleCustomerIDRow{UserID: paddleTestUserID, Status: "active"},
		getOrCreateSub: db.GetUserSubscriptionRow{
			UserID: paddleTestUserID, PlanID: paddleFreePlanID, Status: "active",
		},
	}
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

func TestHandlePaddleWebhook_TransactionCompleted(t *testing.T) {
	m := &paddleMockBilling{
		getPlanByCode: paddleTestPlans,
		getOrCreateSub: db.GetUserSubscriptionRow{
			UserID: paddleTestUserID, PlanID: paddleFreePlanID, Status: "active",
			BillingInterval:    nil,
			CurrentPeriodStart: pgtype.Timestamptz{},
			CancelAtPeriodEnd:  false,
		},
	}
	l := paddleTestLogic(m)

	txnData := map[string]any{
		"id":              "txn_01hgk505qd8mm8yt0yvk1b6w8x",
		"status":          "completed",
		"customer_id":     paddleTestCustomerID,
		"subscription_id": paddleTestSubID,
		"currency_code":   "USD",
		"collection_mode": "automatic",
		"custom_data":     map[string]any{"user_id": paddleTestUserID.String()},
		"items":           []any{map[string]any{"price_id": "pri_01gsz8x8sawmvhz1pv30nge1ke", "quantity": 1}},
	}
	body, sig := paddleBody("transaction.completed", "evt_txn01", txnData)
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	// Upgrade event recorded for the audit trail.
	require.Len(t, m.upgradeEventCalls, 1)
	assert.Equal(t, "checkout_completed", m.upgradeEventCalls[0].EventType)
	assert.Equal(t, "paddle_webhook", m.upgradeEventCalls[0].Surface)
	assert.Equal(t, paddleTestUserID, m.upgradeEventCalls[0].UserID)

	// IDs linked while preserving existing subscription state.
	require.Len(t, m.upsertCalls, 1)
	p := m.upsertCalls[0]
	assert.Equal(t, paddleTestCustomerID, *p.PaddleCustomerID)
	assert.Equal(t, paddleTestSubID, *p.PaddleSubscriptionID)
	assert.Equal(t, paddleFreePlanID, p.PlanID) // preserved — sub event sets the real plan
	assert.Equal(t, []string{"evt_txn01"}, m.markCalls)
}

func TestHandlePaddleWebhook_TransactionCompleted_NoUser(t *testing.T) {
	m := &paddleMockBilling{
		getPlanByCode: paddleTestPlans,
		byCustomerErr: pgx.ErrNoRows,
	}
	l := paddleTestLogic(m)

	txnData := map[string]any{
		"id":          "txn_noUser01",
		"status":      "completed",
		"customer_id": paddleTestCustomerID,
	}
	body, sig := paddleBody("transaction.completed", "evt_txn02", txnData)
	resp, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls)
	assert.Empty(t, m.upgradeEventCalls)
	assert.Equal(t, []string{"evt_txn02"}, m.markCalls)
}

func TestHandlePaddleWebhook_MarkProcessedError_Retries(t *testing.T) {
	m := &paddleMockBilling{
		getPlanByCode:    paddleTestPlans,
		byCustomerRow:    db.GetUserSubscriptionByPaddleCustomerIDRow{UserID: paddleTestUserID, Status: "active"},
		getOrCreateSub:   db.GetUserSubscriptionRow{UserID: paddleTestUserID, PlanID: paddleProPlanID, Status: "active"},
		markProcessedErr: errors.New("db down"),
	}
	l := paddleTestLogic(m)

	body, sig := paddleBody("subscription.updated", "evt_sub10", paddleSubData("active", nil))
	_, err := l.HandlePaddleWebhook(&client.HandlePaddleWebhookRequest{RawBody: body, Signature: sig})
	require.Error(t, err) //nolint:staticcheck // tx rolled back → Paddle retries
}

func TestMapPaddleStatus(t *testing.T) {
	assert.Equal(t, "active", mapPaddleStatus("active"))
	assert.Equal(t, "trialing", mapPaddleStatus("trialing"))
	assert.Equal(t, "past_due", mapPaddleStatus("past_due"))
	assert.Equal(t, "paused", mapPaddleStatus("paused"))
	assert.Equal(t, "canceled", mapPaddleStatus("canceled"))
	assert.Equal(t, "expired", mapPaddleStatus("something_new"))
}
