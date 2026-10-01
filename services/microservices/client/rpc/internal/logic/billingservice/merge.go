package billingservicelogic

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

// Provider-state merge (billing correctness B1+B2).
//
// Each billing provider writes ONLY its own subscription_provider_states row
// (status, period, cancel flag, and the provider-side event watermark
// last_event_at). After every provider-state write the shared `subscriptions`
// row is recomputed from ALL provider states, so a cancel/expiry event from
// one provider can never clobber an active subscription from the other.
//
// Merge rules:
//   - A state "grants" Pro when status ∈ {active, trialing, past_due} AND
//     current_period_end is still in the future (the same check
//     ComputeEntitlements applies, B3).
//   - Among granting states the furthest current_period_end wins — one
//     non-active state can never displace an active one.
//   - When nothing grants, the freshest provider state wins so the row still
//     reflects the latest known truth (e.g. canceled vs expired).

// isGrantingStatus reports whether a provider-reported status is in the
// class that can grant Pro (subject to the period check).
func isGrantingStatus(status string) bool {
	return repository.IsGrantingStatus(status)
}

// grantsPro reports whether a provider state currently entitles the user to
// Pro: a granting status AND a current period that hasn't lapsed.
func grantsPro(status string, periodEnd pgtype.Timestamptz, now time.Time) bool {
	return isGrantingStatus(status) && periodEnd.Valid && periodEnd.Time.After(now)
}

// pickEffectiveState selects the provider state the merged subscriptions row
// should reflect. Returns nil when the user has no provider state rows.
func pickEffectiveState(states []db.SubscriptionProviderState, now time.Time) *db.SubscriptionProviderState {
	var winner *db.SubscriptionProviderState
	for i := range states {
		s := &states[i]
		if !grantsPro(s.Status, s.CurrentPeriodEnd, now) {
			continue
		}
		if winner == nil || laterGrant(s, winner) {
			winner = s
		}
	}
	if winner != nil {
		return winner
	}
	for i := range states {
		s := &states[i]
		if winner == nil || fresher(s, winner) {
			winner = s
		}
	}
	return winner
}

// laterGrant compares two granting states: the one whose paid access ends
// later wins; a tiebreak goes to the state applied more recently.
func laterGrant(a, b *db.SubscriptionProviderState) bool {
	if !a.CurrentPeriodEnd.Time.Equal(b.CurrentPeriodEnd.Time) {
		return a.CurrentPeriodEnd.Time.After(b.CurrentPeriodEnd.Time)
	}
	return eventTime(a).After(eventTime(b))
}

// fresher compares two non-granting states: the one with the most recent
// provider event wins; states without a watermark lose to ones that have it.
func fresher(a, b *db.SubscriptionProviderState) bool {
	ta, tb := eventTime(a), eventTime(b)
	if !ta.Equal(tb) {
		return ta.After(tb)
	}
	if !a.UpdatedAt.Time.Equal(b.UpdatedAt.Time) {
		return a.UpdatedAt.Time.After(b.UpdatedAt.Time)
	}
	// Deterministic fallback so equal states pick the same winner.
	return a.Provider < b.Provider
}

func eventTime(s *db.SubscriptionProviderState) time.Time {
	if s.LastEventAt.Valid {
		return s.LastEventAt.Time
	}
	return time.Time{}
}

// mergedStatus maps the winning provider state onto the merged row's
// status + plan. A granting-class status whose period already lapsed becomes
// 'expired' (B3); 'paused' keeps the pro plan so the UI can show the
// resumable subscription; terminal states map to the free plan.
func mergedStatus(winner *db.SubscriptionProviderState, now time.Time) (status, planCode string) {
	switch {
	case grantsPro(winner.Status, winner.CurrentPeriodEnd, now):
		return winner.Status, "pro"
	case winner.Status == "paused":
		return "paused", "pro"
	case isGrantingStatus(winner.Status):
		return "expired", "free"
	default:
		return winner.Status, "free"
	}
}

// recomputeMergedSubscription reads the user's provider states and rewrites
// the shared subscriptions row as their merged projection. This is the only
// writer of subscription status/plan/period — provider webhooks never touch
// the shared row directly.
func recomputeMergedSubscription(ctx context.Context, repo *repository.Repository, userID uuid.UUID, now time.Time) error {
	states, err := repo.Billing.ListSubscriptionProviderStates(ctx, userID)
	if err != nil {
		return fmt.Errorf("list provider states: %w", err)
	}
	winner := pickEffectiveState(states, now)
	if winner == nil {
		return nil // no provider state yet — leave the (default free) row alone
	}

	status, planCode := mergedStatus(winner, now)
	plan, err := repo.Billing.GetPlanByCode(ctx, planCode)
	if err != nil {
		return fmt.Errorf("get %s plan: %w", planCode, err)
	}

	// The subscriptions_active_has_period CHECK requires interval + periods
	// for active/trialing. Granting implies a valid period end; coalesce the
	// rest defensively.
	interval := winner.BillingInterval
	periodStart := winner.CurrentPeriodStart
	if isGrantingStatus(status) {
		if interval == nil {
			fallback := "monthly"
			interval = &fallback
		}
		if !periodStart.Valid {
			periodStart = winner.LastEventAt
			if !periodStart.Valid {
				periodStart = pgtype.Timestamptz{Time: now, Valid: true}
			}
		}
	}

	var paddleCustomerID, paddleSubscriptionID, revenuecatCustomerID *string
	for i := range states {
		switch states[i].Provider {
		case "paddle":
			paddleCustomerID = states[i].ProviderCustomerID
			paddleSubscriptionID = states[i].ProviderSubscriptionID
		case "revenuecat":
			revenuecatCustomerID = states[i].ProviderCustomerID
		}
	}

	_, err = repo.Billing.ApplyMergedSubscription(ctx, db.ApplyMergedSubscriptionParams{
		UserID:               userID,
		PlanID:               plan.ID,
		Status:               status,
		BillingInterval:      interval,
		CurrentPeriodStart:   periodStart,
		CurrentPeriodEnd:     winner.CurrentPeriodEnd,
		TrialEnd:             winner.TrialEnd,
		CancelAtPeriodEnd:    winner.CancelAtPeriodEnd,
		PaddleCustomerID:     paddleCustomerID,
		PaddleSubscriptionID: paddleSubscriptionID,
		RevenuecatCustomerID: revenuecatCustomerID,
	})
	if err != nil {
		return fmt.Errorf("apply merged subscription: %w", err)
	}
	return nil
}
