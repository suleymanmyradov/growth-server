package billingservicelogic

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

// mockBilling is a stateful in-memory IBilling for webhook tests. It mimics
// the SQL semantics that matter to the handlers: provider-state COALESCE on
// link columns, the GREATEST last_event_at watermark, and recorded checkout
// lookup — so merge/stale/linking behavior is exercised for real.
type mockBilling struct {
	plans         map[string]db.Plan
	getSub        db.GetUserSubscriptionRow
	getSubErr     error
	states        map[string]*db.SubscriptionProviderState // key: userID|provider
	byCustomer    map[string]uuid.UUID                     // paddle customer_id → user
	byCustomerErr error
	checkouts     map[string]uuid.UUID // transaction_id → user
	merged        map[uuid.UUID]db.ApplyMergedSubscriptionParams

	paddleProcessed  map[string]bool
	rcProcessed      map[string]bool
	allProcessed     bool // treat every event as already processed
	isProcessedErr   error
	markProcessedErr error
	upsertErr        error
	checkoutErr      error
	applyErr         error
	upgradeEventErr  error

	upsertCalls     []db.UpsertSubscriptionProviderStateParams
	applyCalls      []db.ApplyMergedSubscriptionParams
	linkCalls       []linkPaddleCall
	checkoutRecords []recordedCheckout
	upgradeEvents   []db.CreateUpgradeEventParams
	paddleMarkCalls []string
	rcMarkCalls     []string
	byCustomerCalls int
}

type linkPaddleCall struct {
	userID         uuid.UUID
	customerID     *string
	subscriptionID *string
	lastEventAt    pgtype.Timestamptz
}

type recordedCheckout struct {
	transactionID string
	userID        uuid.UUID
}

func newMockBilling() *mockBilling {
	return &mockBilling{
		states:          map[string]*db.SubscriptionProviderState{},
		byCustomer:      map[string]uuid.UUID{},
		checkouts:       map[string]uuid.UUID{},
		merged:          map[uuid.UUID]db.ApplyMergedSubscriptionParams{},
		paddleProcessed: map[string]bool{},
		rcProcessed:     map[string]bool{},
	}
}

func stateKey(userID uuid.UUID, provider string) string {
	return fmt.Sprintf("%s|%s", userID, provider)
}

// seedState installs a provider state row (as if a previous webhook wrote it).
func (m *mockBilling) seedState(s db.SubscriptionProviderState) {
	cp := s
	m.states[stateKey(s.UserID, s.Provider)] = &cp
}

// paddleState returns the current stored paddle provider state for a user.
func (m *mockBilling) paddleState(userID uuid.UUID) *db.SubscriptionProviderState {
	return m.states[stateKey(userID, "paddle")]
}

// rcState returns the current stored revenuecat provider state for a user.
func (m *mockBilling) rcState(userID uuid.UUID) *db.SubscriptionProviderState {
	return m.states[stateKey(userID, "revenuecat")]
}

// lastMerged returns the last ApplyMergedSubscription params for a user.
func (m *mockBilling) lastMerged(userID uuid.UUID) db.ApplyMergedSubscriptionParams {
	return m.merged[userID]
}

// ─── Plans / subscriptions (mostly stubs) ───────────────────────────────────

func (m *mockBilling) ListActivePlans(context.Context) ([]db.Plan, error) { panic("not used") }

func (m *mockBilling) GetPlanByCode(_ context.Context, code string) (db.Plan, error) {
	if p, ok := m.plans[code]; ok {
		return p, nil
	}
	return db.Plan{}, errors.New("plan not found")
}

func (m *mockBilling) GetUserSubscription(context.Context, uuid.UUID) (db.GetUserSubscriptionRow, error) {
	if m.getSubErr != nil {
		return db.GetUserSubscriptionRow{}, m.getSubErr
	}
	return m.getSub, nil
}

func (m *mockBilling) GetOrCreateUserSubscription(context.Context, uuid.UUID) (db.GetUserSubscriptionRow, error) {
	return m.GetUserSubscription(context.Background(), uuid.Nil)
}

func (m *mockBilling) CreateDefaultFreeSubscription(context.Context, uuid.UUID) (db.Subscription, error) {
	panic("not used")
}

func (m *mockBilling) CreateUpgradeEvent(_ context.Context, params db.CreateUpgradeEventParams) (db.CreateUpgradeEventRow, error) {
	m.upgradeEvents = append(m.upgradeEvents, params)
	if m.upgradeEventErr != nil {
		return db.CreateUpgradeEventRow{}, m.upgradeEventErr
	}
	return db.CreateUpgradeEventRow{}, nil
}

func (m *mockBilling) ComputeEntitlements(context.Context, db.GetUserSubscriptionRow, uuid.UUID) (*repository.EntitlementsResult, error) {
	panic("not used")
}

func (m *mockBilling) EntitlementsOrFreeFallback(context.Context, uuid.UUID) (*repository.EntitlementsResult, error) {
	panic("not used")
}

func (m *mockBilling) ListSubscriptionStatuses(context.Context) ([]db.ListSubscriptionStatusesRow, error) {
	panic("not used")
}

// ─── Provider states + merged projection ────────────────────────────────────

func (m *mockBilling) GetSubscriptionProviderState(_ context.Context, userID uuid.UUID, provider string) (db.SubscriptionProviderState, error) {
	if s, ok := m.states[stateKey(userID, provider)]; ok {
		return *s, nil
	}
	return db.SubscriptionProviderState{}, pgx.ErrNoRows
}

func (m *mockBilling) ListSubscriptionProviderStates(_ context.Context, userID uuid.UUID) ([]db.SubscriptionProviderState, error) {
	var out []db.SubscriptionProviderState
	for _, s := range m.states {
		if s.UserID == userID {
			out = append(out, *s)
		}
	}
	return out, nil
}

func (m *mockBilling) UpsertSubscriptionProviderState(_ context.Context, p db.UpsertSubscriptionProviderStateParams) (db.SubscriptionProviderState, error) {
	m.upsertCalls = append(m.upsertCalls, p)
	if m.upsertErr != nil {
		return db.SubscriptionProviderState{}, m.upsertErr
	}
	key := stateKey(p.UserID, p.Provider)
	s, ok := m.states[key]
	if !ok {
		s = &db.SubscriptionProviderState{UserID: p.UserID, Provider: p.Provider}
		m.states[key] = s
	}
	s.Status = p.Status
	s.BillingInterval = p.BillingInterval
	s.CurrentPeriodStart = p.CurrentPeriodStart
	s.CurrentPeriodEnd = p.CurrentPeriodEnd
	s.TrialEnd = p.TrialEnd
	s.CancelAtPeriodEnd = p.CancelAtPeriodEnd
	if p.ProviderCustomerID != nil {
		s.ProviderCustomerID = p.ProviderCustomerID
	}
	if p.ProviderSubscriptionID != nil {
		s.ProviderSubscriptionID = p.ProviderSubscriptionID
	}
	// GREATEST: watermark only moves forward; NULL input keeps it.
	if p.LastEventAt.Valid && (!s.LastEventAt.Valid || p.LastEventAt.Time.After(s.LastEventAt.Time)) {
		s.LastEventAt = p.LastEventAt
	}
	if p.LastEventID != nil {
		s.LastEventID = p.LastEventID
	}
	return *s, nil
}

func (m *mockBilling) ApplyMergedSubscription(_ context.Context, p db.ApplyMergedSubscriptionParams) (db.Subscription, error) {
	m.applyCalls = append(m.applyCalls, p)
	if m.applyErr != nil {
		return db.Subscription{}, m.applyErr
	}
	m.merged[p.UserID] = p
	return db.Subscription{UserID: p.UserID, PlanID: p.PlanID, Status: p.Status}, nil
}

func (m *mockBilling) LinkPaddleProviderIDs(_ context.Context, userID uuid.UUID, customerID, subscriptionID *string, lastEventAt pgtype.Timestamptz) error {
	m.linkCalls = append(m.linkCalls, linkPaddleCall{userID, customerID, subscriptionID, lastEventAt})
	key := stateKey(userID, "paddle")
	s, ok := m.states[key]
	if !ok {
		s = &db.SubscriptionProviderState{UserID: userID, Provider: "paddle", Status: "free"}
		m.states[key] = s
	}
	if customerID != nil {
		s.ProviderCustomerID = customerID
	}
	if subscriptionID != nil {
		s.ProviderSubscriptionID = subscriptionID
	}
	if lastEventAt.Valid && (!s.LastEventAt.Valid || lastEventAt.Time.After(s.LastEventAt.Time)) {
		s.LastEventAt = lastEventAt
	}
	return nil
}

// ─── Recorded checkouts ─────────────────────────────────────────────────────

func (m *mockBilling) RecordPaddleCheckout(_ context.Context, transactionID string, userID uuid.UUID) error {
	m.checkoutRecords = append(m.checkoutRecords, recordedCheckout{transactionID, userID})
	if m.checkoutErr != nil {
		return m.checkoutErr
	}
	m.checkouts[transactionID] = userID
	return nil
}

func (m *mockBilling) GetPaddleCheckoutUserID(_ context.Context, transactionID string) (uuid.UUID, error) {
	if uid, ok := m.checkouts[transactionID]; ok {
		return uid, nil
	}
	return uuid.Nil, pgx.ErrNoRows
}

// ─── RevenueCat ─────────────────────────────────────────────────────────────

func (m *mockBilling) GetUserSubscriptionByUserID(context.Context, uuid.UUID) (db.GetUserSubscriptionByUserIDRow, error) {
	panic("not used")
}

func (m *mockBilling) IsRevenueCatEventProcessed(_ context.Context, eventID string) (bool, error) {
	if m.isProcessedErr != nil {
		return false, m.isProcessedErr
	}
	return m.allProcessed || m.rcProcessed[eventID], nil
}

func (m *mockBilling) MarkRevenueCatEventProcessed(_ context.Context, eventID string) error {
	m.rcMarkCalls = append(m.rcMarkCalls, eventID)
	if m.markProcessedErr != nil {
		return m.markProcessedErr
	}
	m.rcProcessed[eventID] = true
	return nil
}

// ─── Paddle ─────────────────────────────────────────────────────────────────

func (m *mockBilling) GetUserSubscriptionByPaddleCustomerID(_ context.Context, customerID *string) (db.GetUserSubscriptionByPaddleCustomerIDRow, error) {
	m.byCustomerCalls++
	if m.byCustomerErr != nil {
		return db.GetUserSubscriptionByPaddleCustomerIDRow{}, m.byCustomerErr
	}
	if customerID == nil {
		return db.GetUserSubscriptionByPaddleCustomerIDRow{}, pgx.ErrNoRows
	}
	if uid, ok := m.byCustomer[*customerID]; ok {
		return db.GetUserSubscriptionByPaddleCustomerIDRow{UserID: uid}, nil
	}
	return db.GetUserSubscriptionByPaddleCustomerIDRow{}, pgx.ErrNoRows
}

func (m *mockBilling) IsPaddleEventProcessed(_ context.Context, eventID string) (bool, error) {
	if m.isProcessedErr != nil {
		return false, m.isProcessedErr
	}
	return m.allProcessed || m.paddleProcessed[eventID], nil
}

func (m *mockBilling) MarkPaddleEventProcessed(_ context.Context, eventID string) error {
	m.paddleMarkCalls = append(m.paddleMarkCalls, eventID)
	if m.markProcessedErr != nil {
		return m.markProcessedErr
	}
	m.paddleProcessed[eventID] = true
	return nil
}

// noopTxRunner is a test-only transaction runner that calls fn directly
// without a real database transaction. It passes a nil pgx.Tx — the logic's
// getTxRepo returns the mock repo whenever a testTxRunner is set.
type noopTxRunner struct{}

func (noopTxRunner) RunSerializable(_ context.Context, _ string, fn func(pgx.Tx) error) error {
	return fn(nil)
}
