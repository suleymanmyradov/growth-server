package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/services/microservices/billing-reconciler/internal/repository"
)

// mockBillingRepo implements billingRepo for testing.
type mockBillingRepo struct {
	expiredSubs    []repository.ListExpiredActiveSubscriptionsRow
	expiredSubsErr error

	freePlan    repository.Plan
	freePlanErr error

	upsertErr error

	// Call capture
	upsertCalls []repository.UpsertUserSubscriptionParams
}

func (m *mockBillingRepo) ListExpiredActiveSubscriptions(_ context.Context, limit int32) ([]repository.ListExpiredActiveSubscriptionsRow, error) {
	return m.expiredSubs, m.expiredSubsErr
}

func (m *mockBillingRepo) GetPlanByCode(_ context.Context, code string) (repository.Plan, error) {
	if m.freePlanErr != nil {
		return repository.Plan{}, m.freePlanErr
	}
	if m.freePlan.Code == code {
		return m.freePlan, nil
	}
	return repository.Plan{}, errors.New("plan not found")
}

func (m *mockBillingRepo) UpsertUserSubscription(_ context.Context, params repository.UpsertUserSubscriptionParams) (repository.UserSubscription, error) {
	m.upsertCalls = append(m.upsertCalls, params)
	return repository.UserSubscription{}, m.upsertErr
}

// --- Tests ---

func TestReconcileExpiredSubscriptions_NoExpiredSubs(t *testing.T) {
	m := &mockBillingRepo{
		expiredSubs: nil,
		freePlan:    repository.Plan{ID: uuid.New(), Code: "free"},
	}

	count, err := reconcileExpiredSubscriptions(context.Background(), m, false)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
	assert.Empty(t, m.upsertCalls)
}

func TestReconcileExpiredSubscriptions_DowngradesToFree(t *testing.T) {
	freePlanID := uuid.New()
	userID := uuid.New()
	pastTime := pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true}

	m := &mockBillingRepo{
		expiredSubs: []repository.ListExpiredActiveSubscriptionsRow{
			{
				UserID:            userID,
				Status:            repository.SubscriptionStatusTypeActive,
				CurrentPeriodEnd:  pastTime,
				CancelAtPeriodEnd: true,
				BillingInterval:   billingIntervalPtr(repository.BillingIntervalTypeMonthly),
			},
		},
		freePlan: repository.Plan{ID: freePlanID, Code: "free"},
	}

	count, err := reconcileExpiredSubscriptions(context.Background(), m, false)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	require.Len(t, m.upsertCalls, 1)
	assert.Equal(t, userID, m.upsertCalls[0].UserID)
	assert.Equal(t, freePlanID, m.upsertCalls[0].PlanID)
	assert.Equal(t, repository.SubscriptionStatusTypeExpired, m.upsertCalls[0].Status)
	assert.False(t, m.upsertCalls[0].CancelAtPeriodEnd)
}

func TestReconcileExpiredSubscriptions_DryRunDoesNotUpsert(t *testing.T) {
	freePlanID := uuid.New()
	userID := uuid.New()
	pastTime := pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true}

	m := &mockBillingRepo{
		expiredSubs: []repository.ListExpiredActiveSubscriptionsRow{
			{
				UserID:            userID,
				Status:            repository.SubscriptionStatusTypeActive,
				CurrentPeriodEnd:  pastTime,
				CancelAtPeriodEnd: true,
			},
		},
		freePlan: repository.Plan{ID: freePlanID, Code: "free"},
	}

	count, err := reconcileExpiredSubscriptions(context.Background(), m, true) // dry=true
	require.NoError(t, err)
	// Dry run logs but doesn't count as reconciled (the `continue` skips the increment)
	assert.Equal(t, 0, count)
	// No upsert calls in dry-run mode
	assert.Empty(t, m.upsertCalls)
}

func TestReconcileExpiredSubscriptions_MultipleExpired(t *testing.T) {
	freePlanID := uuid.New()
	pastTime := pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true}

	m := &mockBillingRepo{
		expiredSubs: []repository.ListExpiredActiveSubscriptionsRow{
			{UserID: uuid.New(), Status: repository.SubscriptionStatusTypeActive, CurrentPeriodEnd: pastTime, CancelAtPeriodEnd: true},
			{UserID: uuid.New(), Status: repository.SubscriptionStatusTypeTrialing, CurrentPeriodEnd: pastTime, CancelAtPeriodEnd: true},
			{UserID: uuid.New(), Status: repository.SubscriptionStatusTypeActive, CurrentPeriodEnd: pastTime, CancelAtPeriodEnd: true},
		},
		freePlan: repository.Plan{ID: freePlanID, Code: "free"},
	}

	count, err := reconcileExpiredSubscriptions(context.Background(), m, false)
	require.NoError(t, err)
	assert.Equal(t, 3, count)
	assert.Len(t, m.upsertCalls, 3)

	// All should be downgraded to free with expired status
	for _, call := range m.upsertCalls {
		assert.Equal(t, freePlanID, call.PlanID)
		assert.Equal(t, repository.SubscriptionStatusTypeExpired, call.Status)
		assert.False(t, call.CancelAtPeriodEnd)
	}
}

func TestReconcileExpiredSubscriptions_ListError(t *testing.T) {
	m := &mockBillingRepo{
		expiredSubsErr: errors.New("database connection lost"),
		freePlan:       repository.Plan{ID: uuid.New(), Code: "free"},
	}

	count, err := reconcileExpiredSubscriptions(context.Background(), m, false)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Empty(t, m.upsertCalls)
}

func TestReconcileExpiredSubscriptions_FreePlanNotFound(t *testing.T) {
	pastTime := pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true}
	m := &mockBillingRepo{
		expiredSubs: []repository.ListExpiredActiveSubscriptionsRow{
			{UserID: uuid.New(), Status: repository.SubscriptionStatusTypeActive, CurrentPeriodEnd: pastTime, CancelAtPeriodEnd: true},
		},
		freePlanErr: errors.New("plan not found"),
	}

	count, err := reconcileExpiredSubscriptions(context.Background(), m, false)
	assert.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Empty(t, m.upsertCalls)
}

func TestReconcileExpiredSubscriptions_UpsertErrorContinues(t *testing.T) {
	freePlanID := uuid.New()
	pastTime := pgtype.Timestamptz{Time: time.Now().Add(-24 * time.Hour), Valid: true}

	m := &mockBillingRepo{
		expiredSubs: []repository.ListExpiredActiveSubscriptionsRow{
			{UserID: uuid.New(), Status: repository.SubscriptionStatusTypeActive, CurrentPeriodEnd: pastTime, CancelAtPeriodEnd: true},
			{UserID: uuid.New(), Status: repository.SubscriptionStatusTypeActive, CurrentPeriodEnd: pastTime, CancelAtPeriodEnd: true},
		},
		freePlan:  repository.Plan{ID: freePlanID, Code: "free"},
		upsertErr: errors.New("write conflict"),
	}

	count, err := reconcileExpiredSubscriptions(context.Background(), m, false)
	// Should not fail — upsert errors are logged and skipped
	require.NoError(t, err)
	assert.Equal(t, 0, count)       // none succeeded
	assert.Len(t, m.upsertCalls, 2) // but both were attempted
}

// --- helpers ---

func billingIntervalPtr(b repository.BillingIntervalType) *repository.BillingIntervalType {
	return &b
}
