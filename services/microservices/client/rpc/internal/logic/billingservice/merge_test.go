package billingservicelogic

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

func providerState(provider, status string, periodEnd, lastEvent time.Time) db.SubscriptionProviderState {
	s := db.SubscriptionProviderState{
		UserID:   uuid.New(),
		Provider: provider,
		Status:   status,
	}
	if !periodEnd.IsZero() {
		s.CurrentPeriodEnd = pgtype.Timestamptz{Time: periodEnd, Valid: true}
	}
	if !lastEvent.IsZero() {
		s.LastEventAt = pgtype.Timestamptz{Time: lastEvent, Valid: true}
	}
	return s
}

func TestGrantsPro(t *testing.T) {
	now := time.Now()
	future := pgtype.Timestamptz{Time: now.Add(time.Hour), Valid: true}
	past := pgtype.Timestamptz{Time: now.Add(-time.Hour), Valid: true}

	// Granting statuses with a live period grant.
	for _, s := range []string{"active", "trialing", "past_due"} {
		assert.True(t, grantsPro(s, future, now), s)
	}
	// Granting status but lapsed period — no Pro (B3).
	assert.False(t, grantsPro("active", past, now))
	// Non-granting statuses never grant.
	for _, s := range []string{"paused", "canceled", "expired", "free"} {
		assert.False(t, grantsPro(s, future, now), s)
	}
	// No period recorded — can't prove access.
	assert.False(t, grantsPro("active", pgtype.Timestamptz{}, now))
}

func TestPickEffectiveState_ActiveBeatsExpired(t *testing.T) {
	now := time.Now()
	states := []db.SubscriptionProviderState{
		// Fresher but dead.
		providerState("paddle", "canceled", now.Add(-time.Hour), now),
		// Older but still granting.
		providerState("revenuecat", "active", now.Add(24*time.Hour), now.Add(-time.Hour)),
	}
	w := pickEffectiveState(states, now)
	require.NotNil(t, w)
	assert.Equal(t, "revenuecat", w.Provider, "active provider must never be displaced by a dead one")
}

func TestPickEffectiveState_FurthestPeriodWins(t *testing.T) {
	now := time.Now()
	states := []db.SubscriptionProviderState{
		providerState("paddle", "active", now.Add(30*24*time.Hour), now),
		providerState("revenuecat", "trialing", now.Add(90*24*time.Hour), now.Add(-time.Hour)),
	}
	w := pickEffectiveState(states, now)
	assert.Equal(t, "revenuecat", w.Provider)
}

func TestPickEffectiveState_NoGranting_FreshestWins(t *testing.T) {
	now := time.Now()
	states := []db.SubscriptionProviderState{
		providerState("paddle", "expired", now.Add(-48*time.Hour), now.Add(-48*time.Hour)),
		providerState("revenuecat", "canceled", now.Add(-24*time.Hour), now.Add(-time.Hour)),
	}
	w := pickEffectiveState(states, now)
	assert.Equal(t, "revenuecat", w.Provider)
}

func TestPickEffectiveState_Empty(t *testing.T) {
	assert.Nil(t, pickEffectiveState(nil, time.Now()))
}

func TestMergedStatus(t *testing.T) {
	now := time.Now()

	// Granting → same status, pro.
	st, plan := mergedStatus(ptr(providerState("paddle", "trialing", now.Add(time.Hour), time.Time{})), now)
	assert.Equal(t, "trialing", st)
	assert.Equal(t, "pro", plan)

	// Paused → keeps pro plan (resumable) but non-granting status.
	st, plan = mergedStatus(ptr(providerState("paddle", "paused", now.Add(time.Hour), time.Time{})), now)
	assert.Equal(t, "paused", st)
	assert.Equal(t, "pro", plan)

	// Granting class but lapsed → expired, free.
	st, plan = mergedStatus(ptr(providerState("paddle", "active", now.Add(-time.Hour), time.Time{})), now)
	assert.Equal(t, "expired", st)
	assert.Equal(t, "free", plan)

	// Terminal states pass through with free plan.
	for _, s := range []string{"canceled", "expired", "free"} {
		st, plan = mergedStatus(ptr(providerState("paddle", s, time.Time{}, time.Time{})), now)
		assert.Equal(t, s, st)
		assert.Equal(t, "free", plan)
	}
}

func ptr(s db.SubscriptionProviderState) *db.SubscriptionProviderState { return &s }
