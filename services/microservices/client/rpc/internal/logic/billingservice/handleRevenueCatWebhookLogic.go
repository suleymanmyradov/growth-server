package billingservicelogic

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/pkg/revenuecat"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// txRunner is the interface the webhook logic uses to run atomic transactions.
// *postgres.PgxTxRunner implements this. Tests can inject a no-op runner.
type txRunner interface {
	RunSerializable(ctx context.Context, userID string, fn func(pgx.Tx) error) error
}

type HandleRevenueCatWebhookLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
	// testTxRunner is only set in tests to inject a no-op transaction runner.
	// In production this is nil and the service context's TxRunner is used.
	testTxRunner txRunner
}

func NewHandleRevenueCatWebhookLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HandleRevenueCatWebhookLogic {
	return &HandleRevenueCatWebhookLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// getTxRunner returns the transaction runner to use. In production this is the
// service context's PgxTxRunner. In tests, a no-op runner can be injected via
// the testTxRunner field.
func (l *HandleRevenueCatWebhookLogic) getTxRunner() txRunner {
	if l.testTxRunner != nil {
		return l.testTxRunner
	}
	return l.svcCtx.TxRunner
}

// getTxRepo returns the repository to use inside a transaction. In production
// this creates a new repository backed by the transaction. In tests (where
// testTxRunner is a no-op that passes nil tx), this returns the mock repo.
func (l *HandleRevenueCatWebhookLogic) getTxRepo(tx pgx.Tx) *repository.Repository {
	if l.testTxRunner != nil {
		return l.svcCtx.Repo
	}
	return l.svcCtx.WithTx(tx)
}

// HandleRevenueCatWebhook processes RevenueCat webhook events for mobile
// subscription lifecycle changes (App Store / Play Store). The gateway forwards
// the raw body + Authorization header; this handler verifies the signature,
// parses the events, and updates the user's subscription accordingly.
//
// Event handling (each event writes ONLY the 'revenuecat' row of
// subscription_provider_states; the shared subscriptions row is then
// recomputed as a merged projection — see merge.go, billing correctness B1):
//   - INITIAL_PURCHASE / RENEWAL / UNCANCELLATION / NON_RENEWING_PURCHASE:
//     provider state 'active' (or 'trialing' when period_type=TRIAL), with
//     period dates from purchased_at_ms / expiration_at_ms.
//   - CANCELLATION: UNSUBSCRIBE et al → cancel_at_period_end (access until
//     expiration); CUSTOMER_SUPPORT is the store refund case → expired now;
//     BILLING_ERROR → past_due.
//   - BILLING_ISSUE: past_due with the store grace-period end when present.
//   - SUBSCRIPTION_PAUSED: provider state 'paused' (Play Store pause — the
//     relationship survives, paid access does not).
//   - EXPIRATION: provider state 'expired'.
//   - PRODUCT_CHANGE / ENTITLEMENT_CHANGE / TRANSFER / SUBSCRIPTION_EXTENDED /
//     REFUND_REVERSED: re-fetch entitlements from the RevenueCat API and
//     converge the provider state to the entitlement truth.
//
// Out-of-order protection (B2): event_timestamp_ms is stored as the provider
// state's last_event_at watermark; an event older than the last applied one
// is skipped so a delayed CANCELLATION can't roll back a newer RENEWAL.
//
// The app_user_id in the webhook is our user UUID (set by Purchases.logIn() on
// the mobile client). Idempotency: each event is processed at most once using
// billing_webhook_events (consumer = 'revenuecat_webhooks'). If an event has
// no id, one is derived from type + app_user_id + timestamp + transaction.
func (l *HandleRevenueCatWebhookLogic) HandleRevenueCatWebhook(in *client.HandleRevenueCatWebhookRequest) (*client.HandleRevenueCatWebhookResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "HandleRevenueCatWebhookLogic.HandleRevenueCatWebhook")
	defer span.End()

	if in == nil || len(in.RawBody) == 0 {
		return nil, status.Error(codes.InvalidArgument, "empty request body")
	}

	cfg := l.svcCtx.Config.Billing.RevenueCat
	if !cfg.Enabled || cfg.WebhookSecret == "" {
		l.Errorf("RevenueCat webhook not configured")
		return nil, status.Error(codes.FailedPrecondition, "revenuecat webhook not configured")
	}

	// Verify the Authorization header against the webhook secret.
	if !revenuecat.VerifyWebhookSignature(in.Authorization, cfg.WebhookSecret) {
		l.Errorf("RevenueCat webhook signature verification failed")
		return nil, status.Error(codes.Unauthenticated, "invalid webhook signature")
	}

	// Parse the webhook payload.
	payload, err := revenuecat.ParseWebhookPayload(in.RawBody)
	if err != nil {
		l.Errorf("RevenueCat webhook parse failed: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid webhook payload")
	}

	events := payload.AllEvents()
	if len(events) == 0 {
		l.Infof("RevenueCat webhook: no events in payload")
		return &client.HandleRevenueCatWebhookResponse{Processed: true}, nil
	}

	// Delivery is unordered — sort by event_timestamp_ms so within a single
	// payload the latest state wins instead of payload order deciding.
	sort.SliceStable(events, func(i, j int) bool {
		return events[i].EventTimestampMs < events[j].EventTimestampMs
	})

	processed := 0
	failed := 0
	for _, evt := range events {
		// Derive a stable event ID for idempotency.
		eventID := evt.EventID
		if eventID == "" {
			eventID = deriveEventID(evt)
		}

		// Process each event in a serializable transaction so that the
		// idempotency check, entitlement mutation, and mark-processed are
		// atomic. If any step fails, the entire transaction rolls back and
		// RevenueCat can safely retry the webhook without double-processing.
		runner := l.getTxRunner()
		err := runner.RunSerializable(ctx, "", func(tx pgx.Tx) error {
			txRepo := l.getTxRepo(tx)

			// Idempotency check inside the transaction.
			alreadyProcessed, err := txRepo.Billing.IsRevenueCatEventProcessed(ctx, eventID)
			if err != nil {
				return fmt.Errorf("idempotency check: %w", err)
			}
			if alreadyProcessed {
				return nil // already processed, skip
			}

			if err := l.handleEventWithRepo(ctx, txRepo, evt); err != nil {
				// Distinguish permanent failures (bad data) from retryable
				// failures (DB errors, transient issues). Permanent failures
				// are marked as processed so RevenueCat doesn't retry them
				// endlessly; retryable failures cause the transaction to
				// roll back and the webhook to return non-2xx.
				if isPermanentEventError(err) {
					l.Errorf("RevenueCat event %s (type=%s user=%s) permanently failed, marking as processed: %v",
						eventID, evt.Type, evt.AppUserID, err)
					billingWebhookPermanentFailuresTotal.WithLabelValues("revenuecat", evt.Type).Inc()
					// Mark as processed so RevenueCat doesn't retry a bad event.
					if markErr := txRepo.Billing.MarkRevenueCatEventProcessed(ctx, eventID); markErr != nil {
						return fmt.Errorf("mark processed (permanent failure): %w", markErr)
					}
					return nil
				}
				return err
			}

			if err := txRepo.Billing.MarkRevenueCatEventProcessed(ctx, eventID); err != nil {
				return fmt.Errorf("mark processed: %w", err)
			}
			return nil
		})

		if err != nil {
			l.Errorf("RevenueCat event %s (type=%s user=%s) failed: %v",
				eventID, evt.Type, evt.AppUserID, err)
			failed++
			continue
		}
		processed++
	}

	l.Infof("RevenueCat webhook: %d/%d events processed, %d failed", processed, len(events), failed)

	// If any event had a retryable failure, return a non-2xx response so
	// RevenueCat retries the entire webhook. Events that were already marked
	// processed (including permanent failures) will be skipped on retry
	// (idempotency). Events that had retryable failures will be re-attempted.
	// Do NOT return success (HTTP 200) when there are retryable failures —
	// RevenueCat only retries on non-2xx responses.
	if failed > 0 {
		return nil, status.Error(codes.Internal, fmt.Sprintf("revenuecat webhook: %d/%d events failed", failed, len(events)))
	}

	return &client.HandleRevenueCatWebhookResponse{Processed: true}, nil
}

// isPermanentEventError returns true for errors that represent bad data or
// invalid input — these will never succeed on retry, so they should be marked
// as processed to prevent infinite retries. Retryable errors (DB failures,
// network issues) return false.
func isPermanentEventError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// Invalid user ID (not a UUID) — RevenueCat sent a bad app_user_id.
	if strings.Contains(msg, "invalid app_user_id") {
		return true
	}
	return false
}

// handleEventWithRepo processes a single RevenueCat webhook event: it writes
// the 'revenuecat' provider state row, then recomputes the merged
// subscriptions row. The provider row is the only thing this provider ever
// writes — another provider's active state can never be clobbered by a
// RevenueCat cancel/expiry (B1), and event_timestamp_ms ordering drops
// delayed events (B2).
func (l *HandleRevenueCatWebhookLogic) handleEventWithRepo(ctx context.Context, repo *repository.Repository, evt revenuecat.WebhookEvent) error {
	userID, err := uuid.Parse(evt.AppUserID)
	if err != nil {
		return fmt.Errorf("invalid app_user_id %q: %w", evt.AppUserID, err)
	}
	// uuid.Parse accepts the all-zero UUID, but no user has it — writing it
	// would create a phantom billing row (seen in prod from a test event).
	if userID == uuid.Nil {
		return fmt.Errorf("invalid app_user_id %q: nil user id", evt.AppUserID)
	}

	prev, err := repo.Billing.GetSubscriptionProviderState(ctx, userID, "revenuecat")
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("get provider state: %w", err)
	}
	state := stateOrNil(prev, err)

	// Out-of-order guard (B2): an event older than the last one applied must
	// not roll the state back.
	if isStaleEvent(state, evt.OccurredAt()) {
		l.Infof("Stale RevenueCat event ignored: type=%s occurred=%s last_applied=%s",
			evt.Type, evt.OccurredAt(), state.LastEventAt.Time)
		return nil
	}

	switch evt.Type {
	case "INITIAL_PURCHASE", "RENEWAL", "UNCANCELLATION", "NON_RENEWING_PURCHASE":
		err = l.handleRCPurchase(ctx, repo, userID, state, evt)
	case "CANCELLATION":
		err = l.handleRCCancellation(ctx, repo, userID, state, evt)
	case "SUBSCRIPTION_PAUSED":
		err = l.writeRCState(ctx, repo, userID, state, evt, "paused", rcTimeMax(evt.ExpirationTime(), statePeriodEnd(state)), true)
	case "BILLING_ISSUE":
		// Grace-period expiry (when present) is when paid access really ends;
		// falling back to expiration, else keep the stored end.
		periodEnd := rcTimeMax(evt.GracePeriodExpiration(), evt.ExpirationTime())
		err = l.writeRCState(ctx, repo, userID, state, evt, "past_due", periodEnd, stateCancelFlag(state))
	case "EXPIRATION":
		periodEnd := evt.ExpirationTime()
		if periodEnd.IsZero() {
			periodEnd = evt.OccurredAt()
		}
		err = l.writeRCState(ctx, repo, userID, state, evt, "expired", periodEnd, false)
		if err == nil {
			l.recordRCBillingEvent(ctx, repo, userID, "subscription_expired", evt)
		}
	case "PRODUCT_CHANGE", "ENTITLEMENT_CHANGE", "TRANSFER", "SUBSCRIPTION_EXTENDED", "REFUND_REVERSED":
		// Ambiguous states — converge to the entitlement truth via the API.
		err = l.handleRCEntitlementResync(ctx, repo, userID, evt)
	default:
		l.Infof("RevenueCat event type %s not handled, skipping", evt.Type)
		return nil
	}
	if err != nil {
		return err
	}

	if err := recomputeMergedSubscription(ctx, repo, userID, time.Now()); err != nil {
		return fmt.Errorf("recompute merged subscription: %w", err)
	}
	return nil
}

// handleRCPurchase writes an active (or trialing) provider state for purchase
// and renewal events.
func (l *HandleRevenueCatWebhookLogic) handleRCPurchase(ctx context.Context, repo *repository.Repository, userID uuid.UUID, state *db.SubscriptionProviderState, evt revenuecat.WebhookEvent) error {
	status := "active"
	var trialEnd pgtype.Timestamptz
	if evt.IsTrial() {
		status = "trialing"
		trialEnd = ts(evt.ExpirationTime())
	}

	periodStart := evt.PurchasedTime()
	if periodStart.IsZero() && state != nil && state.CurrentPeriodStart.Valid {
		periodStart = state.CurrentPeriodStart.Time
	}
	periodEnd := evt.ExpirationTime()
	if periodEnd.IsZero() && state != nil && state.CurrentPeriodEnd.Valid {
		periodEnd = state.CurrentPeriodEnd.Time
	}

	var eventIDPtr *string
	if evt.EventID != "" {
		eventIDPtr = &evt.EventID
	}
	_, err := repo.Billing.UpsertSubscriptionProviderState(ctx, db.UpsertSubscriptionProviderStateParams{
		UserID:                 userID,
		Provider:               "revenuecat",
		Status:                 status,
		BillingInterval:        rcInterval(evt.ProductID),
		CurrentPeriodStart:     ts(periodStart),
		CurrentPeriodEnd:       ts(periodEnd),
		TrialEnd:               trialEnd,
		CancelAtPeriodEnd:      false,
		ProviderCustomerID:     &evt.AppUserID,
		ProviderSubscriptionID: rcSubID(evt, state),
		LastEventAt:            ts(evt.OccurredAt()),
		LastEventID:            eventIDPtr,
	})
	if err != nil {
		return fmt.Errorf("upsert provider state (purchase): %w", err)
	}

	// Funnel event for a user-initiated store purchase — parity with Paddle's
	// checkout_completed (B5 audit trail).
	if evt.Type == "INITIAL_PURCHASE" {
		l.recordRCBillingEvent(ctx, repo, userID, "checkout_completed", evt)
	}

	l.Infof("RevenueCat: user %s subscription activated (status=%s product=%s)", userID, status, evt.ProductID)
	return nil
}

// handleRCCancellation maps CANCELLATION by cancel_reason. CUSTOMER_SUPPORT
// means support granted a refund — access ends immediately (B5). BILLING_ERROR
// means the store stopped the sub over a payment problem — past_due. The rest
// (UNSUBSCRIBE, DEVELOPER_INITIATED, PRICE_INCREASE, UNKNOWN) keep access
// until the current period ends.
func (l *HandleRevenueCatWebhookLogic) handleRCCancellation(ctx context.Context, repo *repository.Repository, userID uuid.UUID, state *db.SubscriptionProviderState, evt revenuecat.WebhookEvent) error {
	switch evt.CancelReason {
	case "CUSTOMER_SUPPORT":
		// Store refund — revoke now, record the audit event (B5).
		if err := l.writeRCState(ctx, repo, userID, state, evt, "expired", evt.OccurredAt(), false); err != nil {
			return err
		}
		l.recordRCBillingEvent(ctx, repo, userID, "subscription_refunded", evt)
		l.Infof("RevenueCat: user %s refunded via %s (customer support) — access revoked", userID, evt.Store)
		return nil
	case "BILLING_ERROR":
		return l.writeRCState(ctx, repo, userID, state, evt, "past_due",
			rcTimeMax(evt.GracePeriodExpiration(), evt.ExpirationTime(), statePeriodEnd(state)), true)
	default:
		// Voluntary cancel — keep the status, flag the pending end.
		status := "active"
		if state != nil && isGrantingStatus(state.Status) {
			status = state.Status
		}
		periodEnd := evt.ExpirationTime()
		if periodEnd.IsZero() {
			periodEnd = statePeriodEnd(state)
		}
		return l.writeRCState(ctx, repo, userID, state, evt, status, periodEnd, true)
	}
}

// writeRCState performs the common provider-state upsert for events that keep
// most fields from the previous state.
func (l *HandleRevenueCatWebhookLogic) writeRCState(ctx context.Context, repo *repository.Repository, userID uuid.UUID, state *db.SubscriptionProviderState, evt revenuecat.WebhookEvent, status string, periodEnd time.Time, cancelAtPeriodEnd bool) error {
	var interval *string
	var periodStart, trialEnd pgtype.Timestamptz
	if state != nil {
		interval = state.BillingInterval
		periodStart = state.CurrentPeriodStart
		if state.Status == "trialing" && status == "trialing" {
			trialEnd = state.TrialEnd
		}
	}
	if iv := rcInterval(evt.ProductID); iv != nil {
		interval = iv
	}
	if periodEnd.IsZero() && state != nil {
		periodEnd = statePeriodEnd(state)
	}

	var eventIDPtr *string
	if evt.EventID != "" {
		eventIDPtr = &evt.EventID
	}
	_, err := repo.Billing.UpsertSubscriptionProviderState(ctx, db.UpsertSubscriptionProviderStateParams{
		UserID:                 userID,
		Provider:               "revenuecat",
		Status:                 status,
		BillingInterval:        interval,
		CurrentPeriodStart:     periodStart,
		CurrentPeriodEnd:       ts(periodEnd),
		TrialEnd:               trialEnd,
		CancelAtPeriodEnd:      cancelAtPeriodEnd,
		ProviderCustomerID:     &evt.AppUserID,
		ProviderSubscriptionID: rcSubID(evt, state),
		LastEventAt:            ts(evt.OccurredAt()),
		LastEventID:            eventIDPtr,
	})
	if err != nil {
		return fmt.Errorf("upsert provider state (%s): %w", status, err)
	}
	l.Infof("RevenueCat: user %s provider state → %s (type=%s reason=%s)", userID, status, evt.Type, evt.CancelReason)
	return nil
}

// handleRCEntitlementResync re-fetches the customer's entitlements from the
// RevenueCat API and converges the provider state to the entitlement truth.
// Used for events whose effect can't be derived from the payload alone
// (PRODUCT_CHANGE, ENTITLEMENT_CHANGE, TRANSFER, SUBSCRIPTION_EXTENDED,
// REFUND_REVERSED).
func (l *HandleRevenueCatWebhookLogic) handleRCEntitlementResync(ctx context.Context, repo *repository.Repository, userID uuid.UUID, evt revenuecat.WebhookEvent) error {
	cfg := l.svcCtx.Config.Billing.RevenueCat
	if cfg.APIKey == "" || cfg.ProjectID == "" {
		// Without API credentials we can't re-fetch. Keep the current state —
		// the next webhook with a clear type will fix it.
		l.Infof("RevenueCat: skipping entitlement re-fetch (no API key configured) for user %s", userID)
		return nil
	}

	rcClient := revenuecat.NewClient(cfg.APIKey, cfg.ProjectID, nil)
	entitlements, err := rcClient.GetCustomerEntitlements(ctx, userID.String())
	if err != nil {
		return fmt.Errorf("fetch entitlements: %w", err)
	}

	prev, err := repo.Billing.GetSubscriptionProviderState(ctx, userID, "revenuecat")
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("get provider state: %w", err)
	}
	state := stateOrNil(prev, err)

	var pro *revenuecat.Entitlement
	for i := range entitlements {
		if entitlements[i].ID == "pro" && entitlements[i].IsActive {
			pro = &entitlements[i]
			break
		}
	}

	if pro != nil {
		var periodEnd time.Time
		if pro.ExpirationDate != nil {
			periodEnd = *pro.ExpirationDate
		}
		return l.writeRCStateAt(ctx, repo, userID, state, evt, "active", pro.PurchaseDate, periodEnd, rcInterval(pro.ProductID), false)
	}

	// No active pro entitlement → expired now.
	return l.writeRCStateAt(ctx, repo, userID, state, evt, "expired", time.Time{}, time.Now(), nil, false)
}

// writeRCStateAt upserts the provider state from an API-fetched entitlement.
// The fetch happens after the triggering event, so its watermark is now —
// the entity subsumes every webhook event that happened before the fetch.
func (l *HandleRevenueCatWebhookLogic) writeRCStateAt(ctx context.Context, repo *repository.Repository, userID uuid.UUID, state *db.SubscriptionProviderState, evt revenuecat.WebhookEvent, status string, periodStart, periodEnd time.Time, interval *string, cancelAtPeriodEnd bool) error {
	if state != nil {
		if interval == nil {
			interval = state.BillingInterval
		}
		if periodStart.IsZero() {
			periodStart = state.CurrentPeriodStart.Time
		}
	}
	var eventIDPtr *string
	if evt.EventID != "" {
		eventIDPtr = &evt.EventID
	}
	_, err := repo.Billing.UpsertSubscriptionProviderState(ctx, db.UpsertSubscriptionProviderStateParams{
		UserID:                 userID,
		Provider:               "revenuecat",
		Status:                 status,
		BillingInterval:        interval,
		CurrentPeriodStart:     ts(periodStart),
		CurrentPeriodEnd:       ts(periodEnd),
		CancelAtPeriodEnd:      cancelAtPeriodEnd,
		ProviderCustomerID:     &evt.AppUserID,
		ProviderSubscriptionID: rcSubID(evt, state),
		LastEventAt:            ts(time.Now()),
		LastEventID:            eventIDPtr,
	})
	if err != nil {
		return fmt.Errorf("upsert provider state (resync %s): %w", status, err)
	}
	l.Infof("RevenueCat: user %s provider state resynced → %s (type=%s)", userID, status, evt.Type)
	return nil
}

// recordRCBillingEvent writes an upgrade_events audit row for RevenueCat
// billing changes; failures are logged and non-fatal.
func (l *HandleRevenueCatWebhookLogic) recordRCBillingEvent(ctx context.Context, repo *repository.Repository, userID uuid.UUID, eventType string, evt revenuecat.WebhookEvent) {
	_, err := repo.Billing.CreateUpgradeEvent(ctx, db.CreateUpgradeEventParams{
		UserID:    userID,
		EventType: eventType,
		Surface:   "revenuecat_webhook",
		Code:      "pro",
		Metadata: repository.NullJSON(map[string]any{
			"event_id":           evt.EventID,
			"type":               evt.Type,
			"store":              evt.Store,
			"product_id":         evt.ProductID,
			"cancel_reason":      evt.CancelReason,
			"expiration_reason":  evt.ExpirationReason,
			"transaction_id":     evt.TransactionID,
			"original_txn_id":    evt.OriginalTransactionID,
			"event_timestamp_ms": evt.EventTimestampMs,
		}),
	})
	if err != nil {
		l.Errorf("Failed to record %s event: %v", eventType, err)
	}
}

// statePeriodEnd returns the stored provider state's period end (zero if none).
func statePeriodEnd(s *db.SubscriptionProviderState) time.Time {
	if s == nil || !s.CurrentPeriodEnd.Valid {
		return time.Time{}
	}
	return s.CurrentPeriodEnd.Time
}

// stateCancelFlag returns the stored provider state's cancel flag.
func stateCancelFlag(s *db.SubscriptionProviderState) bool {
	return s != nil && s.CancelAtPeriodEnd
}

// rcSubID picks the provider-side subscription identifier to store:
// original_transaction_id is stable across renewals; falls back to the
// stored value when the event doesn't carry one.
func rcSubID(evt revenuecat.WebhookEvent, state *db.SubscriptionProviderState) *string {
	if evt.OriginalTransactionID != "" {
		return &evt.OriginalTransactionID
	}
	if state != nil {
		return state.ProviderSubscriptionID
	}
	return nil
}

// rcInterval maps a RevenueCat product id to a billing interval. Unknown or
// empty product ids return nil so the stored interval is preserved.
func rcInterval(productID string) *string {
	p := strings.ToLower(productID)
	switch {
	case p == "":
		return nil
	case strings.Contains(p, "annual") || strings.Contains(p, "yearly") || strings.Contains(p, "year"):
		v := "annual"
		return &v
	default:
		v := "monthly"
		return &v
	}
}

// rcTimeMax returns the latest non-zero time.
func rcTimeMax(times ...time.Time) time.Time {
	var max time.Time
	for _, t := range times {
		if t.After(max) {
			max = t
		}
	}
	return max
}

// deriveEventID builds a stable event ID when RevenueCat doesn't send one
// (retries reuse the same event_timestamp_ms, so the derivation is stable).
func deriveEventID(evt revenuecat.WebhookEvent) string {
	raw := fmt.Sprintf("%s:%s:%s:%d:%s", evt.Type, evt.AppUserID, evt.ProductID, evt.EventTimestampMs, evt.TransactionID)
	return fmt.Sprintf("rc_%x", hashString(raw))
}

func hashString(s string) uint64 {
	h := uint64(1469598103934665603)
	for _, c := range s {
		h ^= uint64(c)
		h *= 1099511628211
	}
	return h
}
