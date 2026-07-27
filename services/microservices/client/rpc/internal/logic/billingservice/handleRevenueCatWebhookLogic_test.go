package billingservicelogic

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
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

// rcMockBilling is a configurable mock for the RevenueCat webhook tests.
type rcMockBilling struct {
	getOrCreateSub    db.GetUserSubscriptionRow
	getOrCreateErr    error
	getPlanByCode     map[string]db.Plan
	upsertResult      db.Subscription
	upsertErr         error
	setRcCustomerErr  error
	isProcessed       bool
	isProcessedErr    error
	markProcessedErr  error
	setRcCustomerCalls int
	upsertCalls       []db.UpsertUserSubscriptionParams
}

func (m *rcMockBilling) ListActivePlans(ctx context.Context) ([]db.Plan, error) { panic("not used") }
func (m *rcMockBilling) GetPlanByCode(_ context.Context, code string) (db.Plan, error) {
	if p, ok := m.getPlanByCode[code]; ok {
		return p, nil
	}
	return db.Plan{}, errors.New("plan not found")
}
func (m *rcMockBilling) GetUserSubscription(ctx context.Context, userID uuid.UUID) (db.GetUserSubscriptionRow, error) {
	panic("not used")
}
func (m *rcMockBilling) GetOrCreateUserSubscription(ctx context.Context, userID uuid.UUID) (db.GetUserSubscriptionRow, error) {
	if m.getOrCreateErr != nil {
		return db.GetUserSubscriptionRow{}, m.getOrCreateErr
	}
	return m.getOrCreateSub, nil
}
func (m *rcMockBilling) GetUserSubscriptionByStripeCustomerID(ctx context.Context, stripeCustomerID *string) (db.GetUserSubscriptionByStripeCustomerIDRow, error) {
	panic("not used")
}
func (m *rcMockBilling) CreateDefaultFreeSubscription(ctx context.Context, userID uuid.UUID) (db.Subscription, error) {
	panic("not used")
}
func (m *rcMockBilling) UpsertUserSubscription(_ context.Context, params db.UpsertUserSubscriptionParams) (db.Subscription, error) {
	m.upsertCalls = append(m.upsertCalls, params)
	if m.upsertErr != nil {
		return db.Subscription{}, m.upsertErr
	}
	return m.upsertResult, nil
}
func (m *rcMockBilling) CreateUpgradeEvent(ctx context.Context, params db.CreateUpgradeEventParams) (db.CreateUpgradeEventRow, error) {
	panic("not used")
}
func (m *rcMockBilling) ComputeEntitlements(ctx context.Context, sub db.GetUserSubscriptionRow, userID uuid.UUID) (*repository.EntitlementsResult, error) {
	panic("not used")
}
func (m *rcMockBilling) IsStripeEventProcessed(ctx context.Context, stripeEventID string) (bool, error) {
	panic("not used")
}
func (m *rcMockBilling) MarkStripeEventProcessed(ctx context.Context, stripeEventID string) error {
	panic("not used")
}
func (m *rcMockBilling) ListExpiredActiveSubscriptions(ctx context.Context, limit int32) ([]db.ListExpiredActiveSubscriptionsRow, error) {
	panic("not used")
}
func (m *rcMockBilling) ListSubscriptionStatuses(ctx context.Context) ([]db.ListSubscriptionStatusesRow, error) {
	panic("not used")
}
func (m *rcMockBilling) GetUserSubscriptionByUserID(ctx context.Context, userID uuid.UUID) (db.GetUserSubscriptionByUserIDRow, error) {
	panic("not used")
}
func (m *rcMockBilling) SetRevenueCatCustomerID(_ context.Context, userID uuid.UUID, _ *string) error {
	m.setRcCustomerCalls++
	return m.setRcCustomerErr
}
func (m *rcMockBilling) IsRevenueCatEventProcessed(_ context.Context, _ string) (bool, error) {
	if m.isProcessedErr != nil {
		return false, m.isProcessedErr
	}
	return m.isProcessed, nil
}
func (m *rcMockBilling) MarkRevenueCatEventProcessed(_ context.Context, _ string) error {
	return m.markProcessedErr
}

// noopTxRunner is a test-only transaction runner that calls fn directly
// without a real database transaction. It passes a nil pgx.Tx — the test
// logic uses getTxRepo which returns the mock repo when testTxRunner is set.
type noopTxRunner struct{}

func (noopTxRunner) RunSerializable(_ context.Context, _ string, fn func(pgx.Tx) error) error {
	return fn(nil)
}

func rcTestLogic(m *rcMockBilling) *HandleRevenueCatWebhookLogic {
	return &HandleRevenueCatWebhookLogic{
		ctx:    context.Background(),
		svcCtx: &svc.ServiceContext{
			Config: config.Config{},
			Repo:   &repository.Repository{Billing: m},
		},
		Logger:        logx.WithContext(context.Background()),
		testTxRunner:  noopTxRunner{},
	}
}

func rcWebhookBody(events ...map[string]any) []byte {
	payload := map[string]any{"events": events}
	b, _ := json.Marshal(payload)
	return b
}

func TestHandleRevenueCatWebhook_NotConfigured(t *testing.T) {
	m := &rcMockBilling{}
	l := rcTestLogic(m)
	l.svcCtx.Config.Billing.RevenueCat.Enabled = false

	_, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       rcWebhookBody(),
		Authorization: "Bearer secret",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.FailedPrecondition, st.Code())
}

func TestHandleRevenueCatWebhook_InvalidSignature(t *testing.T) {
	m := &rcMockBilling{}
	l := rcTestLogic(m)
	l.svcCtx.Config.Billing.RevenueCat.Enabled = true
	l.svcCtx.Config.Billing.RevenueCat.WebhookSecret = "correct-secret"

	_, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       rcWebhookBody(),
		Authorization: "Bearer wrong-secret",
	})
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())
}

func TestHandleRevenueCatWebhook_EmptyBody(t *testing.T) {
	m := &rcMockBilling{}
	l := rcTestLogic(m)
	l.svcCtx.Config.Billing.RevenueCat.Enabled = true
	l.svcCtx.Config.Billing.RevenueCat.WebhookSecret = "secret"

	_, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       nil,
		Authorization: "Bearer secret",
	})
	require.Error(t, err)
	st, _ := status.FromError(err)
	assert.Equal(t, codes.InvalidArgument, st.Code())
}

func TestHandleRevenueCatWebhook_NoEvents(t *testing.T) {
	m := &rcMockBilling{}
	l := rcTestLogic(m)
	l.svcCtx.Config.Billing.RevenueCat.Enabled = true
	l.svcCtx.Config.Billing.RevenueCat.WebhookSecret = "secret"

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       rcWebhookBody(),
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
}

func TestHandleRevenueCatWebhook_InitialPurchase(t *testing.T) {
	userID := uuid.New()
	planID := uuid.New()
	m := &rcMockBilling{
		getOrCreateSub: db.GetUserSubscriptionRow{
			UserID: userID,
			PlanID: planID,
			Status: "free",
			PlanCode: "free",
		},
		getPlanByCode: map[string]db.Plan{
			"pro": {ID: planID, Code: "pro"},
		},
	}
	l := rcTestLogic(m)
	l.svcCtx.Config.Billing.RevenueCat.Enabled = true
	l.svcCtx.Config.Billing.RevenueCat.WebhookSecret = "secret"

	body := rcWebhookBody(map[string]any{
		"type":            "INITIAL_PURCHASE",
		"store":           "APP_STORE",
		"app_user_id":     userID.String(),
		"product_id":      "com.growth.pro.monthly",
		"entitlement_id":  "pro",
		"period_start_at": "2025-07-22T00:00:00Z",
		"expiration_at":   "2025-08-22T00:00:00Z",
	})

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	// Verify the subscription was upserted with pro plan + active status.
	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "active", m.upsertCalls[0].Status)
	assert.Equal(t, planID, m.upsertCalls[0].PlanID)
	assert.False(t, m.upsertCalls[0].CancelAtPeriodEnd)

	// Verify RevenueCat customer ID was linked.
	assert.Equal(t, 1, m.setRcCustomerCalls)
}

func TestHandleRevenueCatWebhook_Expiration(t *testing.T) {
	userID := uuid.New()
	proPlanID := uuid.New()
	freePlanID := uuid.New()
	m := &rcMockBilling{
		getOrCreateSub: db.GetUserSubscriptionRow{
			UserID: userID,
			PlanID: proPlanID,
			Status: "active",
			PlanCode: "pro",
		},
		getPlanByCode: map[string]db.Plan{
			"free": {ID: freePlanID, Code: "free"},
		},
	}
	l := rcTestLogic(m)
	l.svcCtx.Config.Billing.RevenueCat.Enabled = true
	l.svcCtx.Config.Billing.RevenueCat.WebhookSecret = "secret"

	body := rcWebhookBody(map[string]any{
		"type":        "EXPIRATION",
		"store":       "PLAY_STORE",
		"app_user_id": userID.String(),
		"product_id":  "com.growth.pro.annual",
	})

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	// Verify the subscription was downgraded to free + expired.
	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "expired", m.upsertCalls[0].Status)
	assert.Equal(t, freePlanID, m.upsertCalls[0].PlanID)
}

func TestHandleRevenueCatWebhook_Cancellation(t *testing.T) {
	userID := uuid.New()
	planID := uuid.New()
	m := &rcMockBilling{
		getOrCreateSub: db.GetUserSubscriptionRow{
			UserID: userID,
			PlanID: planID,
			Status: "active",
			PlanCode: "pro",
			BillingInterval: strPtr("monthly"),
			CurrentPeriodEnd: pgtype.Timestamptz{Valid: true},
		},
	}
	l := rcTestLogic(m)
	l.svcCtx.Config.Billing.RevenueCat.Enabled = true
	l.svcCtx.Config.Billing.RevenueCat.WebhookSecret = "secret"

	body := rcWebhookBody(map[string]any{
		"type":          "CANCELLATION",
		"store":         "APP_STORE",
		"app_user_id":   userID.String(),
		"product_id":    "com.growth.pro.monthly",
		"expiration_at": "2025-08-22T00:00:00Z",
	})

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	assert.True(t, m.upsertCalls[0].CancelAtPeriodEnd)
	assert.Equal(t, "active", m.upsertCalls[0].Status) // still active until period end
}

func TestHandleRevenueCatWebhook_DuplicateEventSkipped(t *testing.T) {
	userID := uuid.New()
	m := &rcMockBilling{
		getOrCreateSub: db.GetUserSubscriptionRow{UserID: userID, Status: "free", PlanCode: "free"},
		isProcessed:    true, // already processed
	}
	l := rcTestLogic(m)
	l.svcCtx.Config.Billing.RevenueCat.Enabled = true
	l.svcCtx.Config.Billing.RevenueCat.WebhookSecret = "secret"

	body := rcWebhookBody(map[string]any{
		"type":        "INITIAL_PURCHASE",
		"app_user_id": userID.String(),
		"product_id":  "com.growth.pro.monthly",
		"event_id":    "evt-duplicate-1",
	})

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	// No upsert should have been called.
	assert.Empty(t, m.upsertCalls)
}

func TestHandleRevenueCatWebhook_InvalidUserID(t *testing.T) {
	m := &rcMockBilling{
		getOrCreateSub: db.GetUserSubscriptionRow{},
		getOrCreateErr: errors.New("not found"),
	}
	l := rcTestLogic(m)
	l.svcCtx.Config.Billing.RevenueCat.Enabled = true
	l.svcCtx.Config.Billing.RevenueCat.WebhookSecret = "secret"

	body := rcWebhookBody(map[string]any{
		"type":        "INITIAL_PURCHASE",
		"app_user_id": "not-a-uuid",
		"product_id":  "com.growth.pro.monthly",
	})

	// The handler should not fail the whole webhook for one bad event — it
	// logs the error and continues. The response should still be Processed=true.
	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)
	assert.True(t, resp.Processed)
}

func TestHandleRevenueCatWebhook_TrialDetection(t *testing.T) {
	userID := uuid.New()
	planID := uuid.New()
	m := &rcMockBilling{
		getOrCreateSub: db.GetUserSubscriptionRow{UserID: userID, Status: "free", PlanCode: "free"},
		getPlanByCode: map[string]db.Plan{
			"pro": {ID: planID, Code: "pro"},
		},
	}
	l := rcTestLogic(m)
	l.svcCtx.Config.Billing.RevenueCat.Enabled = true
	l.svcCtx.Config.Billing.RevenueCat.WebhookSecret = "secret"

	body := rcWebhookBody(map[string]any{
		"type":            "INITIAL_PURCHASE",
		"app_user_id":     userID.String(),
		"product_id":      "com.growth.pro.trial_monthly",
		"period_start_at": "2026-07-22T00:00:00Z",
		"expiration_at":   "2027-08-22T00:00:00Z",
	})

	_, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "trialing", m.upsertCalls[0].Status)
}

func TestHandleRevenueCatWebhook_AnnualInterval(t *testing.T) {
	userID := uuid.New()
	planID := uuid.New()
	m := &rcMockBilling{
		getOrCreateSub: db.GetUserSubscriptionRow{UserID: userID, Status: "free", PlanCode: "free"},
		getPlanByCode: map[string]db.Plan{
			"pro": {ID: planID, Code: "pro"},
		},
	}
	l := rcTestLogic(m)
	l.svcCtx.Config.Billing.RevenueCat.Enabled = true
	l.svcCtx.Config.Billing.RevenueCat.WebhookSecret = "secret"

	body := rcWebhookBody(map[string]any{
		"type":            "RENEWAL",
		"app_user_id":     userID.String(),
		"product_id":      "com.growth.pro.annual",
		"period_start_at": "2025-07-22T00:00:00Z",
		"expiration_at":   "2026-07-22T00:00:00Z",
	})

	_, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.NoError(t, err)

	require.Len(t, m.upsertCalls, 1)
	interval := m.upsertCalls[0].BillingInterval
	require.NotNil(t, interval)
	assert.Equal(t, "annual", *interval)
}

func TestDeriveEventID(t *testing.T) {
	evt := revenuecat.WebhookEvent{
		Type:          "INITIAL_PURCHASE",
		AppUserID:     "user-1",
		ProductID:     "com.growth.pro.monthly",
		PeriodStartAt: "2025-07-22T00:00:00Z",
	}
	id1 := deriveEventID(evt)
	id2 := deriveEventID(evt)
	assert.Equal(t, id1, id2, "derived event ID should be deterministic")

	// Different event should produce different ID.
	evt.ProductID = "com.growth.pro.annual"
	id3 := deriveEventID(evt)
	assert.NotEqual(t, id1, id3)
}

// TestHandleRevenueCatWebhook_RetryableFailureReturnsError verifies that a
// retryable failure (e.g. DB error during upsert) causes the webhook to return
// a non-2xx error so RevenueCat retries. This is the critical behavior: we must
// NOT return HTTP 200 when there are retryable failures.
func TestHandleRevenueCatWebhook_RetryableFailureReturnsError(t *testing.T) {
	userID := uuid.New()
	planID := uuid.New()
	m := &rcMockBilling{
		getOrCreateSub: db.GetUserSubscriptionRow{
			UserID:   userID,
			PlanID:   planID,
			Status:   "free",
			PlanCode: "free",
		},
		getPlanByCode: map[string]db.Plan{
			"pro": {ID: planID, Code: "pro"},
		},
		upsertErr: errors.New("database connection lost"), // retryable
	}
	l := rcTestLogic(m)
	l.svcCtx.Config.Billing.RevenueCat.Enabled = true
	l.svcCtx.Config.Billing.RevenueCat.WebhookSecret = "secret"

	body := rcWebhookBody(map[string]any{
		"type":        "INITIAL_PURCHASE",
		"app_user_id": userID.String(),
		"product_id":  "com.growth.pro.monthly",
	})

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.Error(t, err, "retryable failure must return error so RevenueCat retries")
	assert.Nil(t, resp, "response must be nil on error")
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
}

// TestHandleRevenueCatWebhook_MarkProcessedFailureReturnsError verifies that
// a failure to mark an event as processed (after successful handling) causes
// the webhook to return a non-2xx error. Without this, a crash after processing
// but before marking could lead to duplicate processing on retry with no
// signal to RevenueCat that retry is needed.
func TestHandleRevenueCatWebhook_MarkProcessedFailureReturnsError(t *testing.T) {
	userID := uuid.New()
	planID := uuid.New()
	m := &rcMockBilling{
		getOrCreateSub: db.GetUserSubscriptionRow{
			UserID:   userID,
			PlanID:   planID,
			Status:   "free",
			PlanCode: "free",
		},
		getPlanByCode: map[string]db.Plan{
			"pro": {ID: planID, Code: "pro"},
		},
		markProcessedErr: errors.New("db error marking processed"),
	}
	l := rcTestLogic(m)
	l.svcCtx.Config.Billing.RevenueCat.Enabled = true
	l.svcCtx.Config.Billing.RevenueCat.WebhookSecret = "secret"

	body := rcWebhookBody(map[string]any{
		"type":        "INITIAL_PURCHASE",
		"app_user_id": userID.String(),
		"product_id":  "com.growth.pro.monthly",
	})

	resp, err := l.HandleRevenueCatWebhook(&client.HandleRevenueCatWebhookRequest{
		RawBody:       body,
		Authorization: "Bearer secret",
	})
	require.Error(t, err, "mark-processed failure must return error so RevenueCat retries")
	assert.Nil(t, resp)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Internal, st.Code())
}

func TestParseTime(t *testing.T) {
	// Valid time.
	ts := parseTime("2025-07-22T00:00:00Z")
	assert.True(t, ts.Valid)

	// Empty string.
	ts = parseTime("")
	assert.False(t, ts.Valid)

	// Invalid format.
	ts = parseTime("not-a-time")
	assert.False(t, ts.Valid)

	// Whitespace-only.
	ts = parseTime("  ")
	assert.False(t, ts.Valid)
}
