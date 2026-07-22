package billingservicelogic

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/logx"
)

// mockBilling is a configurable mock implementing repository.IBilling.
// Only the methods exercised by webhook logic are tracked; the rest panic
// so unintended calls surface immediately in tests.
type mockBilling struct {
	// Configurable return values
	getPlanByCode                   map[string]db.Plan
	getSubByStripeCustomer          map[string]db.GetUserSubscriptionByStripeCustomerIDRow
	getSubByStripeCustomerErr       error
	upsertResult                    db.Subscription
	upsertErr                       error
	createUpgradeEventErr           error
	isStripeEventProcessed          bool
	isStripeEventProcessedErr       error
	markStripeEventProcessedErr     error

	// Call capture for assertions
	upsertCalls             []db.UpsertUserSubscriptionParams
	createUpgradeEventCalls []db.CreateUpgradeEventParams
	markProcessedCalls      []string
	processedCheckCalls     []string
}

func (m *mockBilling) ListActivePlans(ctx context.Context) ([]db.Plan, error) {
	panic("not used in webhook tests")
}
func (m *mockBilling) GetPlanByCode(_ context.Context, code string) (db.Plan, error) {
	if p, ok := m.getPlanByCode[code]; ok {
		return p, nil
	}
	return db.Plan{}, errors.New("plan not found")
}
func (m *mockBilling) GetUserSubscription(ctx context.Context, userID uuid.UUID) (db.GetUserSubscriptionRow, error) {
	panic("not used in webhook tests")
}
func (m *mockBilling) GetOrCreateUserSubscription(ctx context.Context, userID uuid.UUID) (db.GetUserSubscriptionRow, error) {
	panic("not used in webhook tests")
}
func (m *mockBilling) GetUserSubscriptionByStripeCustomerID(_ context.Context, cid *string) (db.GetUserSubscriptionByStripeCustomerIDRow, error) {
	if m.getSubByStripeCustomerErr != nil {
		return db.GetUserSubscriptionByStripeCustomerIDRow{}, m.getSubByStripeCustomerErr
	}
	if s, ok := m.getSubByStripeCustomer[*cid]; ok {
		return s, nil
	}
	return db.GetUserSubscriptionByStripeCustomerIDRow{}, errors.New("no rows")
}
func (m *mockBilling) CreateDefaultFreeSubscription(ctx context.Context, userID uuid.UUID) (db.Subscription, error) {
	panic("not used in webhook tests")
}
func (m *mockBilling) UpsertUserSubscription(_ context.Context, params db.UpsertUserSubscriptionParams) (db.Subscription, error) {
	m.upsertCalls = append(m.upsertCalls, params)
	return m.upsertResult, m.upsertErr
}
func (m *mockBilling) CreateUpgradeEvent(_ context.Context, params db.CreateUpgradeEventParams) (db.CreateUpgradeEventRow, error) {
	m.createUpgradeEventCalls = append(m.createUpgradeEventCalls, params)
	return db.CreateUpgradeEventRow{}, m.createUpgradeEventErr
}
func (m *mockBilling) ComputeEntitlements(ctx context.Context, sub db.GetUserSubscriptionRow, userID uuid.UUID) (*EntitlementsResult, error) {
	panic("not used in webhook tests")
}
func (m *mockBilling) IsStripeEventProcessed(_ context.Context, eventID string) (bool, error) {
	m.processedCheckCalls = append(m.processedCheckCalls, eventID)
	return m.isStripeEventProcessed, m.isStripeEventProcessedErr
}
func (m *mockBilling) MarkStripeEventProcessed(_ context.Context, eventID string) error {
	m.markProcessedCalls = append(m.markProcessedCalls, eventID)
	return m.markStripeEventProcessedErr
}
func (m *mockBilling) ListExpiredActiveSubscriptions(ctx context.Context, limit int32) ([]db.ListExpiredActiveSubscriptionsRow, error) {
	panic("not used in webhook tests")
}
func (m *mockBilling) ListSubscriptionStatuses(ctx context.Context) ([]db.ListSubscriptionStatusesRow, error) {
	panic("not used in webhook tests")
}

// --- Test helpers ---

func newTestLogic(m *mockBilling) *HandleStripeWebhookLogic {
	return &HandleStripeWebhookLogic{
		ctx:    context.Background(),
		svcCtx: &svc.ServiceContext{},
		Logger: logx.WithContext(context.Background()),
	}
}

// withRepo injects a mock billing repository into the logic's svcCtx.
// repository.Repository is a struct with all interface fields, so we can
// construct it with just Billing set and the rest nil — the webhook logic
// only accesses Repo.Billing.
func (l *HandleStripeWebhookLogic) withRepo(m *mockBilling) *HandleStripeWebhookLogic {
	l.svcCtx.Repo = &repository.Repository{Billing: m}
	return l
}

// strPtr returns a pointer to s.
func strPtr(s string) *string { return &s }

// fixedUUIDs for deterministic tests.
var (
	testUserID       = uuid.MustParse("00000000-0000-7000-8000-000000000001")
	testFreePlanID   = uuid.MustParse("00000000-0000-7000-8000-000000000002")
	testProPlanID    = uuid.MustParse("00000000-0000-7000-8000-000000000003")
	testFreePlan     = db.Plan{ID: testFreePlanID, Code: "free", Name: "Free"}
	testProPlan      = db.Plan{ID: testProPlanID, Code: "pro", Name: "Pro"}
	testCustomerID   = "cus_test_123"
	testSubID        = "sub_test_456"
	testExistingSub  = db.GetUserSubscriptionByStripeCustomerIDRow{
		UserID:               testUserID,
		PlanID:               testFreePlanID,
		Status:               "free",
		StripeCustomerID:     strPtr(testCustomerID),
		StripeSubscriptionID: nil,
	}
)

// --- mapStripeStatus tests ---

func TestMapStripeStatus(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"trialing", "trialing"},
		{"active", "active"},
		{"past_due", "past_due"},
		{"canceled", "canceled"},
		{"incomplete_expired", "expired"},
		{"unknown_status", "expired"}, // default
		{"", "expired"},               // empty default
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := mapStripeStatus(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

// --- handleCheckoutCompleted tests ---

func TestHandleCheckoutCompleted_HappyPath(t *testing.T) {
	m := &mockBilling{
		getPlanByCode:          map[string]db.Plan{"pro": testProPlan},
		getSubByStripeCustomer: map[string]db.GetUserSubscriptionByStripeCustomerIDRow{testCustomerID: testExistingSub},
	}
	l := newTestLogic(m).withRepo(m)

	data := mustJSON(t, stripeCheckoutData{
		Object: stripeCheckoutSession{
			ID:           "cs_test_1",
			Customer:     testCustomerID,
			Subscription: testSubID,
		},
	})

	resp, err := l.handleCheckoutCompleted(data)
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	// Should have recorded an upgrade event
	require.Len(t, m.createUpgradeEventCalls, 1)
	assert.Equal(t, "checkout_completed", m.createUpgradeEventCalls[0].EventType)
	assert.Equal(t, "stripe_webhook", m.createUpgradeEventCalls[0].Surface)
	assert.Equal(t, "{}", string(m.createUpgradeEventCalls[0].Metadata))

	// Should have upserted with the subscription ID, keeping existing status
	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, testUserID, m.upsertCalls[0].UserID)
	assert.Equal(t, testProPlanID, m.upsertCalls[0].PlanID)
	assert.Equal(t, "free", m.upsertCalls[0].Status) // not "active" — period dates come later
	assert.Equal(t, testSubID, *m.upsertCalls[0].StripeSubscriptionID)
}

func TestHandleCheckoutCompleted_CustomerNotFound(t *testing.T) {
	m := &mockBilling{
		getPlanByCode:          map[string]db.Plan{"pro": testProPlan},
		getSubByStripeCustomerErr: errors.New("no rows"),
	}
	l := newTestLogic(m).withRepo(m)

	data := mustJSON(t, stripeCheckoutData{
		Object: stripeCheckoutSession{Customer: "cus_unknown", Subscription: "sub_1"},
	})

	resp, err := l.handleCheckoutCompleted(data)
	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Empty(t, m.upsertCalls) // no upsert on failure
}

func TestHandleCheckoutCompleted_NoSubscriptionID(t *testing.T) {
	m := &mockBilling{
		getPlanByCode:          map[string]db.Plan{"pro": testProPlan},
		getSubByStripeCustomer: map[string]db.GetUserSubscriptionByStripeCustomerIDRow{testCustomerID: testExistingSub},
	}
	l := newTestLogic(m).withRepo(m)

	data := mustJSON(t, stripeCheckoutData{
		Object: stripeCheckoutSession{Customer: testCustomerID, Subscription: ""},
	})

	resp, err := l.handleCheckoutCompleted(data)
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	// Upgrade event recorded, but no upsert (no subscription ID)
	assert.Len(t, m.createUpgradeEventCalls, 1)
	assert.Empty(t, m.upsertCalls)
}

// --- handleSubscriptionUpdated tests ---

func TestHandleSubscriptionUpdated_PeriodDatesFromItemLevel(t *testing.T) {
	// In newer Stripe API versions, period dates are on the subscription item, not top-level.
	m := &mockBilling{
		getPlanByCode:          map[string]db.Plan{"pro": testProPlan},
		getSubByStripeCustomer: map[string]db.GetUserSubscriptionByStripeCustomerIDRow{testCustomerID: testExistingSub},
	}
	l := newTestLogic(m).withRepo(m)

	data := mustJSON(t, stripeSubscriptionData{
		Object: stripeSubscription{
			ID:                 testSubID,
			Customer:           testCustomerID,
			Status:             "active",
			CurrentPeriodStart: 0, // top-level is 0 (new API version)
			CurrentPeriodEnd:   0,
			Items: struct {
				Data []stripeSubscriptionItem `json:"data"`
			}{
				Data: []stripeSubscriptionItem{
					{
						Price:              stripePrice{ID: "price_1", Recurring: struct{ Interval string `json:"interval"` }{Interval: "month"}},
						CurrentPeriodStart: 1700000000,
						CurrentPeriodEnd:   1702678400,
					},
				},
			},
		},
	})

	resp, err := l.handleSubscriptionUpdated(data)
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "active", m.upsertCalls[0].Status)
	assert.True(t, m.upsertCalls[0].CurrentPeriodStart.Valid)
	assert.True(t, m.upsertCalls[0].CurrentPeriodEnd.Valid)
	assert.Equal(t, "monthly", *m.upsertCalls[0].BillingInterval)
}

func TestHandleSubscriptionUpdated_PeriodDatesFromTopLevel(t *testing.T) {
	// Older Stripe API versions have period dates on the subscription top-level.
	m := &mockBilling{
		getPlanByCode:          map[string]db.Plan{"pro": testProPlan},
		getSubByStripeCustomer: map[string]db.GetUserSubscriptionByStripeCustomerIDRow{testCustomerID: testExistingSub},
	}
	l := newTestLogic(m).withRepo(m)

	data := mustJSON(t, stripeSubscriptionData{
		Object: stripeSubscription{
			ID:                 testSubID,
			Customer:           testCustomerID,
			Status:             "active",
			CurrentPeriodStart: 1700000000,
			CurrentPeriodEnd:   1702678400,
			Items: struct {
				Data []stripeSubscriptionItem `json:"data"`
			}{
				Data: []stripeSubscriptionItem{
					{Price: stripePrice{ID: "price_1", Recurring: struct{ Interval string `json:"interval"` }{Interval: "year"}}},
				},
			},
		},
	})

	resp, err := l.handleSubscriptionUpdated(data)
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	assert.True(t, m.upsertCalls[0].CurrentPeriodStart.Valid)
	assert.Equal(t, "annual", *m.upsertCalls[0].BillingInterval)
}

func TestHandleSubscriptionUpdated_StaleWebhookIgnored(t *testing.T) {
	// DB has a different subscription ID than the webhook — should be ignored.
	staleSub := testExistingSub
	staleSub.StripeSubscriptionID = strPtr("sub_newer_999")
	m := &mockBilling{
		getPlanByCode:          map[string]db.Plan{"pro": testProPlan},
		getSubByStripeCustomer: map[string]db.GetUserSubscriptionByStripeCustomerIDRow{testCustomerID: staleSub},
	}
	l := newTestLogic(m).withRepo(m)

	data := mustJSON(t, stripeSubscriptionData{
		Object: stripeSubscription{
			ID:       "sub_old_111",
			Customer: testCustomerID,
			Status:   "active",
		},
	})

	resp, err := l.handleSubscriptionUpdated(data)
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls) // no upsert — stale webhook
}

func TestHandleSubscriptionUpdated_CustomerNotFound(t *testing.T) {
	m := &mockBilling{
		getPlanByCode:          map[string]db.Plan{"pro": testProPlan},
		getSubByStripeCustomerErr: errors.New("no rows"),
	}
	l := newTestLogic(m).withRepo(m)

	data := mustJSON(t, stripeSubscriptionData{
		Object: stripeSubscription{ID: testSubID, Customer: "cus_unknown", Status: "active"},
	})

	resp, err := l.handleSubscriptionUpdated(data)
	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- handleSubscriptionDeleted tests ---

func TestHandleSubscriptionDeleted_DowngradesToFree(t *testing.T) {
	proSub := testExistingSub
	proSub.Status = "active"
	proSub.StripeSubscriptionID = strPtr(testSubID)
	m := &mockBilling{
		getPlanByCode:          map[string]db.Plan{"free": testFreePlan},
		getSubByStripeCustomer: map[string]db.GetUserSubscriptionByStripeCustomerIDRow{testCustomerID: proSub},
	}
	l := newTestLogic(m).withRepo(m)

	data := mustJSON(t, stripeSubscriptionData{
		Object: stripeSubscription{ID: testSubID, Customer: testCustomerID},
	})

	resp, err := l.handleSubscriptionDeleted(data)
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, testFreePlanID, m.upsertCalls[0].PlanID)
	assert.Equal(t, "canceled", m.upsertCalls[0].Status)
	assert.False(t, m.upsertCalls[0].CancelAtPeriodEnd)
}

func TestHandleSubscriptionDeleted_StaleWebhookIgnored(t *testing.T) {
	proSub := testExistingSub
	proSub.StripeSubscriptionID = strPtr("sub_newer_999")
	m := &mockBilling{
		getPlanByCode:          map[string]db.Plan{"free": testFreePlan},
		getSubByStripeCustomer: map[string]db.GetUserSubscriptionByStripeCustomerIDRow{testCustomerID: proSub},
	}
	l := newTestLogic(m).withRepo(m)

	data := mustJSON(t, stripeSubscriptionData{
		Object: stripeSubscription{ID: "sub_old_111", Customer: testCustomerID},
	})

	resp, err := l.handleSubscriptionDeleted(data)
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	assert.Empty(t, m.upsertCalls)
}

// --- handlePaymentFailed tests ---

func TestHandlePaymentFailed_SetsPastDue(t *testing.T) {
	proSub := testExistingSub
	proSub.Status = "active"
	proSub.PlanID = testProPlanID
	m := &mockBilling{
		getSubByStripeCustomer: map[string]db.GetUserSubscriptionByStripeCustomerIDRow{testCustomerID: proSub},
	}
	l := newTestLogic(m).withRepo(m)

	data := mustJSON(t, stripeInvoiceData{
		Object: stripeInvoice{
			ID:           "in_test_1",
			Customer:     testCustomerID,
			Subscription: testSubID,
			Status:       "open",
		},
	})

	resp, err := l.handlePaymentFailed(data)
	require.NoError(t, err)
	assert.True(t, resp.Processed)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, "past_due", m.upsertCalls[0].Status)
	assert.Equal(t, testProPlanID, m.upsertCalls[0].PlanID) // keeps current plan
}

func TestHandlePaymentFailed_CustomerNotFound(t *testing.T) {
	m := &mockBilling{
		getSubByStripeCustomerErr: errors.New("no rows"),
	}
	l := newTestLogic(m).withRepo(m)

	data := mustJSON(t, stripeInvoiceData{
		Object: stripeInvoice{Customer: "cus_unknown"},
	})

	resp, err := l.handlePaymentFailed(data)
	assert.Nil(t, resp)
	assert.Error(t, err)
	assert.Empty(t, m.upsertCalls)
}

// --- handleDisputeCreated tests ---

func TestHandleDisputeCreated_JustLogs(t *testing.T) {
	m := &mockBilling{}
	l := newTestLogic(m).withRepo(m)

	data := mustJSON(t, stripeDisputeData{
		Object: stripeDispute{
			ID:            "dp_test_1",
			Charge:        "ch_test_1",
			Amount:        999,
			Currency:      "usd",
			Status:        "needs_response",
			Reason:        "fraudulent",
			PaymentIntent: "pi_test_1",
		},
	})

	resp, err := l.handleDisputeCreated(data)
	require.NoError(t, err)
	assert.True(t, resp.Processed)
	// No DB calls — dispute only logs for now
	assert.Empty(t, m.upsertCalls)
}

// --- Invalid JSON tests ---

func TestHandleCheckoutCompleted_InvalidJSON(t *testing.T) {
	m := &mockBilling{}
	l := newTestLogic(m).withRepo(m)

	resp, err := l.handleCheckoutCompleted(json.RawMessage(`{invalid`))
	assert.Nil(t, resp)
	assert.Error(t, err)
}

func TestHandleSubscriptionUpdated_InvalidJSON(t *testing.T) {
	m := &mockBilling{}
	l := newTestLogic(m).withRepo(m)

	resp, err := l.handleSubscriptionUpdated(json.RawMessage(`{invalid`))
	assert.Nil(t, resp)
	assert.Error(t, err)
}

// --- mustJSON helper ---

func mustJSON(t *testing.T, v interface{}) json.RawMessage {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return b
}

// Ensure pgtype is referenced to avoid unused import if tests evolve.
var _ = pgtype.Timestamptz{}
