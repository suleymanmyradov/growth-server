package billingservicelogic

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"github.com/suleymanmyradov/growth-server/pkg/paddle"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// errPaddleUserNotFound marks events we can never map to a user (transaction
// not created by our checkout endpoint and no existing paddle_customer_id
// link). Retrying the identical payload cannot fix that, so these are marked
// processed rather than burning Paddle's retry budget.
var errPaddleUserNotFound = errors.New("paddle event has no mappable user")

type HandlePaddleWebhookLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
	// testTxRunner is only set in tests to inject a no-op transaction runner.
	testTxRunner txRunner
}

func NewHandlePaddleWebhookLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HandlePaddleWebhookLogic {
	return &HandlePaddleWebhookLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *HandlePaddleWebhookLogic) getTxRunner() txRunner {
	if l.testTxRunner != nil {
		return l.testTxRunner
	}
	return l.svcCtx.TxRunner
}

func (l *HandlePaddleWebhookLogic) getTxRepo(tx pgx.Tx) *repository.Repository {
	if l.testTxRunner != nil {
		return l.svcCtx.Repo
	}
	return l.svcCtx.WithTx(tx)
}

// HandlePaddleWebhook verifies the Paddle-Signature header, dedupes on
// event_id (billing_webhook_events, consumer 'paddle_webhooks'), and mirrors
// subscription state into subscription_provider_states. The shared
// subscriptions row is then recomputed as a merged projection of every
// provider state (see merge.go — B1: a non-active provider state can never
// clobber an active one; B2: last_event_at drops out-of-order deliveries).
//
// Events handled:
//   - subscription.*: convergent upsert of the paddle provider state
//     (status, period, interval, cancel flag from scheduled_change).
//   - transaction.completed: links paddle IDs + records a checkout_completed
//     upgrade event for the audit trail. User resolution trusts
//     custom_data.user_id ONLY for transactions recorded by
//     CreatePaddleCheckout (paddle_checkouts table, B4) — client-side
//     checkouts leave custom_data attacker-controlled.
//   - adjustment.*: approved full refunds and chargebacks revoke access
//     (provider state → expired) and record subscription_refunded /
//     subscription_chargeback; reversals re-fetch state to restore (B5).
//
// User linkage: transaction.completed is resolved via paddle_checkouts first,
// then the stored paddle_customer_id link. subscription.* events resolve via
// the paddle_customer_id link only — custom_data on subscriptions is never
// trusted (Paddle doesn't copy transaction custom_data there anyway).
func (l *HandlePaddleWebhookLogic) HandlePaddleWebhook(in *client.HandlePaddleWebhookRequest) (*client.HandlePaddleWebhookResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "HandlePaddleWebhookLogic.HandlePaddleWebhook")
	defer span.End()

	cfg := l.svcCtx.Config.Billing.Paddle
	if !cfg.Enabled || cfg.WebhookSecret == "" {
		l.Errorf("Paddle webhook not configured")
		return nil, status.Error(codes.FailedPrecondition, "paddle webhook verification not configured")
	}

	if err := paddle.VerifySignature(in.RawBody, in.Signature, cfg.WebhookSecret); err != nil {
		l.Errorf("Paddle webhook verification failed: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid signature")
	}

	event, err := paddle.ParseEvent(in.RawBody)
	if err != nil {
		l.Errorf("Failed to parse paddle event: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid event payload")
	}

	var result *client.HandlePaddleWebhookResponse
	var postCommitSubID string // best-effort API action after commit
	var postCommitCancel bool  // true: cancel at Paddle; false: re-fetch state
	err = l.getTxRunner().RunSerializable(ctx, "", func(tx pgx.Tx) error {
		txRepo := l.getTxRepo(tx)

		if event.EventID != "" {
			processed, err := txRepo.Billing.IsPaddleEventProcessed(ctx, event.EventID)
			if err != nil {
				return fmt.Errorf("idempotency check: %w", err)
			}
			if processed {
				l.Infof("duplicate webhook skipped: %s", event.EventID)
				result = &client.HandlePaddleWebhookResponse{Processed: true}
				return nil
			}
		}

		var handleErr error
		switch {
		case strings.HasPrefix(event.EventType, "subscription."):
			result, handleErr = l.handleSubscriptionEvent(ctx, txRepo, event)
		case event.EventType == paddle.EventTransactionCompleted:
			result, postCommitSubID, handleErr = l.handleTransactionCompleted(ctx, txRepo, event)
		case event.EventType == paddle.EventAdjustmentCreated || event.EventType == paddle.EventAdjustmentUpdated:
			result, postCommitSubID, postCommitCancel, handleErr = l.handleAdjustmentEvent(ctx, txRepo, event)
		default:
			l.Infof("Unhandled webhook event type: %s", event.EventType)
			result = &client.HandlePaddleWebhookResponse{Processed: true}
		}
		if handleErr != nil {
			if errors.Is(handleErr, errPaddleUserNotFound) {
				// Permanent: a retry of the identical payload can never map a
				// user. Mark processed so Paddle stops redelivering, and keep
				// the loud log line for ops.
				l.Errorf("Paddle event %s (type=%s) unprocessable, marking processed: %v",
					event.EventID, event.EventType, handleErr)
				billingWebhookPermanentFailuresTotal.WithLabelValues("paddle", event.EventType).Inc()
				if event.EventID != "" {
					if markErr := txRepo.Billing.MarkPaddleEventProcessed(ctx, event.EventID); markErr != nil {
						return fmt.Errorf("mark paddle event processed (permanent failure): %w", markErr)
					}
				}
				result = &client.HandlePaddleWebhookResponse{Processed: true}
				return nil
			}
			// Retryable: roll the whole transaction back so Paddle retries.
			return handleErr
		}

		if event.EventID != "" {
			if err := txRepo.Billing.MarkPaddleEventProcessed(ctx, event.EventID); err != nil {
				return fmt.Errorf("mark paddle event processed: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Best-effort API actions after commit. transaction.completed only links
	// IDs — the subscription entity itself may have arrived out of order and
	// been skipped, so a direct fetch converges the provider state immediately
	// instead of waiting for the next subscription.* event. For refunds and
	// chargebacks the subscription is canceled at Paddle — a plain re-fetch
	// would see the still-active entity and restore the revoked access.
	if postCommitSubID != "" {
		if postCommitCancel {
			l.cancelPaddleSubscriptionNow(ctx, postCommitSubID)
		} else {
			l.syncPaddleSubscriptionFromAPI(ctx, postCommitSubID)
		}
	}

	return result, nil
}

// handleSubscriptionEvent mirrors a Paddle subscription entity into the user's
// paddle provider state (convergent upsert), then recomputes the merged row.
func (l *HandlePaddleWebhookLogic) handleSubscriptionEvent(ctx context.Context, repo *repository.Repository, event *paddle.Envelope) (*client.HandlePaddleWebhookResponse, error) {
	sub, err := event.Subscription()
	if err != nil {
		l.Errorf("Failed to parse paddle subscription data: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid subscription data")
	}

	userID, err := l.resolveByPaddleCustomer(ctx, repo, sub.CustomerID)
	if err != nil {
		return nil, err
	}
	if claimed := sub.UserID(); claimed != "" && claimed != userID.String() {
		l.Errorf("paddle subscription %s: ignoring custom_data.user_id=%q (resolved user %s via customer link)",
			sub.ID, claimed, userID)
	}

	provState, err := repo.Billing.GetSubscriptionProviderState(ctx, userID, "paddle")
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		l.Errorf("Failed to load paddle provider state for user %s: %v", userID, err)
		return nil, status.Error(codes.Internal, "failed to load subscription")
	}
	state := stateOrNil(provState, err)

	// Stale-subscription guard: events for a superseded subscription id must
	// not clobber the current one. Only applies while the stored state is
	// still plausibly live — after it ends, a different incoming id is a
	// legitimate re-subscription.
	if state != nil && state.ProviderSubscriptionID != nil && *state.ProviderSubscriptionID != sub.ID &&
		providerStateLive(state, time.Now()) {
		l.Infof("Stale subscription update ignored: db_sub=%s webhook_sub=%s customer=%s",
			*state.ProviderSubscriptionID, sub.ID, sub.CustomerID)
		return &client.HandlePaddleWebhookResponse{Processed: true}, nil
	}

	// Out-of-order guard (B2): an event older than the last one applied must
	// not roll the state back.
	if stale := isStaleEvent(state, event.OccurredAt); stale {
		l.Infof("Stale paddle event ignored: type=%s occurred=%s last_applied=%s",
			event.EventType, event.OccurredAt, state.LastEventAt.Time)
		return &client.HandlePaddleWebhookResponse{Processed: true}, nil
	}

	if err := l.applyPaddleSubscriptionEntity(ctx, repo, userID, sub, event.OccurredAt, event.EventID); err != nil {
		return nil, err
	}

	l.Infof("Paddle subscription synced: sub=%s customer=%s status=%s",
		sub.ID, sub.CustomerID, sub.Status)
	return &client.HandlePaddleWebhookResponse{Processed: true}, nil
}

// applyPaddleSubscriptionEntity writes a Paddle subscription entity to the
// provider state and recomputes the merged row. Shared by the webhook path
// and the post-commit API re-sync (which passes a zero eventID and the fetch
// time as occurredAt).
func (l *HandlePaddleWebhookLogic) applyPaddleSubscriptionEntity(ctx context.Context, repo *repository.Repository, userID uuid.UUID, sub *paddle.Subscription, occurredAt time.Time, eventID string) error {
	localStatus := mapPaddleStatus(sub.Status)

	var periodStart, periodEnd pgtype.Timestamptz
	if sub.CurrentBillingPeriod != nil {
		periodStart = ts(sub.CurrentBillingPeriod.StartsAt)
		periodEnd = ts(sub.CurrentBillingPeriod.EndsAt)
	}

	// Paddle subscriptions carry no top-level trial_end; while trialing, the
	// current billing period covers the trial window.
	var trialEnd pgtype.Timestamptz
	if localStatus == "trialing" {
		trialEnd = periodEnd
	}

	var billingInterval *string
	if sub.BillingCycle != nil {
		switch sub.BillingCycle.Interval {
		case "month":
			v := "monthly"
			billingInterval = &v
		case "year":
			v := "annual"
			billingInterval = &v
		}
	}

	// A non-null scheduled_change with action 'cancel' means the user pressed
	// cancel but keeps access until effective_at — the status stays 'active'
	// until then, so the flag (not the status) carries the pending cancel.
	// A scheduled 'pause' ends paid access at effective_at the same way (the
	// subscription stops billing and 'paused' carries no entitlement), so it
	// maps onto the same flag — a pause stays resumable from the Paddle portal.
	cancelAtPeriodEnd := sub.CancelScheduled() || sub.PauseScheduled()

	var eventIDPtr *string
	if eventID != "" {
		eventIDPtr = &eventID
	}

	_, err := repo.Billing.UpsertSubscriptionProviderState(ctx, db.UpsertSubscriptionProviderStateParams{
		UserID:                 userID,
		Provider:               "paddle",
		Status:                 localStatus,
		BillingInterval:        billingInterval,
		CurrentPeriodStart:     periodStart,
		CurrentPeriodEnd:       periodEnd,
		TrialEnd:               trialEnd,
		CancelAtPeriodEnd:      cancelAtPeriodEnd,
		ProviderCustomerID:     &sub.CustomerID,
		ProviderSubscriptionID: &sub.ID,
		LastEventAt:            ts(occurredAt),
		LastEventID:            eventIDPtr,
	})
	if err != nil {
		l.Errorf("Failed to upsert paddle provider state: %v", err)
		return status.Error(codes.Internal, "failed to update subscription")
	}

	if err := recomputeMergedSubscription(ctx, repo, userID, time.Now()); err != nil {
		l.Errorf("Failed to recompute merged subscription: %v", err)
		return status.Error(codes.Internal, "failed to update subscription")
	}
	return nil
}

// handleTransactionCompleted links the Paddle customer/subscription IDs and
// records a checkout_completed upgrade event for backend-created checkouts.
// The subscription.* events carry the full state; this is the audit trail
// for the payment itself plus the B4-trusted user resolution path.
func (l *HandlePaddleWebhookLogic) handleTransactionCompleted(ctx context.Context, repo *repository.Repository, event *paddle.Envelope) (*client.HandlePaddleWebhookResponse, string, error) {
	txn, err := event.Transaction()
	if err != nil {
		l.Errorf("Failed to parse paddle transaction data: %v", err)
		return nil, "", status.Error(codes.InvalidArgument, "invalid transaction data")
	}

	userID, viaCheckout, err := l.resolveTransactionUser(ctx, repo, txn)
	if err != nil {
		return nil, "", err
	}

	// checkout_completed is a funnel event for user-initiated checkouts —
	// only recorded when the transaction was created by our checkout endpoint.
	// Renewal transactions resolve via the customer link and are skipped here.
	if viaCheckout {
		if _, eventErr := repo.Billing.CreateUpgradeEvent(ctx, db.CreateUpgradeEventParams{
			UserID:    userID,
			EventType: "checkout_completed",
			Surface:   "paddle_webhook",
			Code:      "pro",
			Metadata:  []byte("{}"),
		}); eventErr != nil {
			l.Errorf("Failed to record checkout completion event: %v", eventErr)
			// Non-fatal: continue processing
		}
	}

	// Link the Paddle IDs on the provider state — subscription.* events may
	// arrive before OR after this one. The merge then pushes the IDs onto the
	// shared subscriptions row for future lookups.
	if txn.CustomerID != "" || txn.SubscriptionID != "" {
		var subIDPtr *string
		if txn.SubscriptionID != "" {
			subIDPtr = &txn.SubscriptionID
		}
		var customerIDPtr *string
		if txn.CustomerID != "" {
			customerIDPtr = &txn.CustomerID
		}
		if err := repo.Billing.LinkPaddleProviderIDs(ctx, userID, customerIDPtr, subIDPtr, ts(event.OccurredAt)); err != nil {
			l.Errorf("Failed to link paddle IDs: %v", err)
			return nil, "", status.Error(codes.Internal, "failed to update subscription")
		}
		if err := recomputeMergedSubscription(ctx, repo, userID, time.Now()); err != nil {
			l.Errorf("Failed to recompute merged subscription: %v", err)
			return nil, "", status.Error(codes.Internal, "failed to update subscription")
		}
	}

	l.Infof("Paddle transaction completed: txn=%s customer=%s", txn.ID, txn.CustomerID)
	return &client.HandlePaddleWebhookResponse{Processed: true}, txn.SubscriptionID, nil
}

// handleAdjustmentEvent processes Paddle adjustments (B5): approved full
// refunds and chargebacks revoke access immediately; reversals trigger an API
// re-fetch to restore state. Partial refunds, credits, and pending/rejected
// adjustments are logged without revoking.
// Returns the post-commit API intent: subscription id + whether to cancel it
// (true) or just re-fetch its state (false).
func (l *HandlePaddleWebhookLogic) handleAdjustmentEvent(ctx context.Context, repo *repository.Repository, event *paddle.Envelope) (*client.HandlePaddleWebhookResponse, string, bool, error) {
	adj, err := event.Adjustment()
	if err != nil {
		l.Errorf("Failed to parse paddle adjustment data: %v", err)
		return nil, "", false, status.Error(codes.InvalidArgument, "invalid adjustment data")
	}

	switch {
	case adj.IsMoneyBack() && (adj.Action != "refund" || adj.Type == "full"):
		// Approved full refund or chargeback — the customer got their money
		// back (or disputed it), so access ends now.
		userID, err := l.resolveAdjustmentUser(ctx, repo, adj)
		if err != nil {
			return nil, "", false, err
		}
		if err := l.revokePaddleAccess(ctx, repo, userID, adj, event); err != nil {
			return nil, "", false, err
		}
		eventType := "subscription_refunded"
		if adj.Action == "chargeback" {
			eventType = "subscription_chargeback"
		}
		l.recordBillingEvent(ctx, repo, userID, eventType, map[string]any{
			"adjustment_id":  adj.ID,
			"transaction_id": adj.TransactionID,
			"action":         adj.Action,
			"status":         adj.Status,
			"type":           adj.Type,
			"reason":         adj.Reason,
		})
		l.Infof("Paddle %s %s applied: user=%s adj=%s txn=%s — access revoked",
			adj.Action, adj.Status, userID, adj.ID, adj.TransactionID)
		// Ask Paddle to stop the subscription too, so the revoked state
		// converges with a subscription.canceled event rather than lingering.
		return &client.HandlePaddleWebhookResponse{Processed: true}, l.cancelSubIDFor(ctx, repo, userID, adj), true, nil

	case adj.IsReversal() || (adj.IsMoneyBack() && adj.Action == "refund" && adj.Type != "full"):
		// Reversal (dispute won / credit reversed) or partial refund.
		userID, err := l.resolveAdjustmentUser(ctx, repo, adj)
		if err != nil {
			return nil, "", false, err
		}
		if adj.IsReversal() {
			l.recordBillingEvent(ctx, repo, userID, "subscription_restored", map[string]any{
				"adjustment_id":  adj.ID,
				"transaction_id": adj.TransactionID,
				"action":         adj.Action,
			})
			// Re-fetch the subscription so state converges to Paddle's truth —
			// restores access if the subscription is still live.
			subID := l.cancelSubIDFor(ctx, repo, userID, adj)
			l.Infof("Paddle adjustment reversal: user=%s adj=%s — resyncing", userID, adj.ID)
			return &client.HandlePaddleWebhookResponse{Processed: true}, subID, false, nil
		}
		l.Infof("Paddle partial refund (no revoke): user=%s adj=%s txn=%s", userID, adj.ID, adj.TransactionID)
		return &client.HandlePaddleWebhookResponse{Processed: true}, "", false, nil

	default:
		// pending_approval / rejected / reversed / credit / chargeback_warning:
		// nothing to do — adjustment.updated fires on approval.
		l.Infof("Paddle adjustment %s (action=%s status=%s) — no action", adj.ID, adj.Action, adj.Status)
		return &client.HandlePaddleWebhookResponse{Processed: true}, "", false, nil
	}
}

// revokePaddleAccess marks the paddle provider state expired at the event
// time, then recomputes the merged row. Another provider's active state still
// wins the merge — a web refund never strips a live mobile subscription.
func (l *HandlePaddleWebhookLogic) revokePaddleAccess(ctx context.Context, repo *repository.Repository, userID uuid.UUID, adj *paddle.Adjustment, event *paddle.Envelope) error {
	prev, err := repo.Billing.GetSubscriptionProviderState(ctx, userID, "paddle")
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		l.Errorf("Failed to load paddle provider state for revoke: %v", err)
		return status.Error(codes.Internal, "failed to update subscription")
	}

	var interval *string
	var periodStart pgtype.Timestamptz
	customerID := adj.CustomerID
	var subID *string
	if err == nil {
		interval = prev.BillingInterval
		periodStart = prev.CurrentPeriodStart
		if customerID == "" && prev.ProviderCustomerID != nil {
			customerID = *prev.ProviderCustomerID
		}
		subID = prev.ProviderSubscriptionID
	}
	if adj.SubscriptionID != nil && *adj.SubscriptionID != "" {
		subID = adj.SubscriptionID
	}
	var customerIDPtr *string
	if customerID != "" {
		customerIDPtr = &customerID
	}

	_, err = repo.Billing.UpsertSubscriptionProviderState(ctx, db.UpsertSubscriptionProviderStateParams{
		UserID:                 userID,
		Provider:               "paddle",
		Status:                 "expired",
		BillingInterval:        interval,
		CurrentPeriodStart:     periodStart,
		CurrentPeriodEnd:       ts(event.OccurredAt), // access ended at refund
		CancelAtPeriodEnd:      false,
		ProviderCustomerID:     customerIDPtr,
		ProviderSubscriptionID: subID,
		LastEventAt:            ts(event.OccurredAt),
		LastEventID:            &event.EventID,
	})
	if err != nil {
		l.Errorf("Failed to mark paddle provider state revoked: %v", err)
		return status.Error(codes.Internal, "failed to update subscription")
	}

	if err := recomputeMergedSubscription(ctx, repo, userID, time.Now()); err != nil {
		l.Errorf("Failed to recompute merged subscription: %v", err)
		return status.Error(codes.Internal, "failed to update subscription")
	}
	return nil
}

// recordBillingEvent writes an upgrade_events audit row; failures are logged
// and non-fatal (the provider state write already happened).
func (l *HandlePaddleWebhookLogic) recordBillingEvent(ctx context.Context, repo *repository.Repository, userID uuid.UUID, eventType string, metadata map[string]any) {
	_, err := repo.Billing.CreateUpgradeEvent(ctx, db.CreateUpgradeEventParams{
		UserID:    userID,
		EventType: eventType,
		Surface:   "paddle_webhook",
		Metadata:  repository.NullJSON(metadata),
	})
	if err != nil {
		l.Errorf("Failed to record %s event: %v", eventType, err)
	}
}

// cancelSubIDFor picks the subscription id to act on post-commit (API cancel
// or re-fetch): the adjustment's own subscription_id when present, else the
// one already stored on the provider state.
func (l *HandlePaddleWebhookLogic) cancelSubIDFor(ctx context.Context, repo *repository.Repository, userID uuid.UUID, adj *paddle.Adjustment) string {
	if adj.SubscriptionID != nil && *adj.SubscriptionID != "" {
		return *adj.SubscriptionID
	}
	// Look up the stored provider sub id; failures are non-fatal — the
	// post-commit step is best-effort anyway.
	state, err := repo.Billing.GetSubscriptionProviderState(ctx, userID, "paddle")
	if err == nil && state.ProviderSubscriptionID != nil {
		return *state.ProviderSubscriptionID
	}
	return ""
}

// syncPaddleSubscriptionFromAPI fetches the subscription entity and applies
// it as provider state + merged recompute — best effort, all errors logged.
// The fetch time serves as the event watermark: the fetched entity subsumes
// every webhook event that happened before it.
func (l *HandlePaddleWebhookLogic) syncPaddleSubscriptionFromAPI(ctx context.Context, subscriptionID string) {
	cli := l.svcCtx.PaddleClient
	if cli == nil || subscriptionID == "" {
		return
	}
	sub, err := cli.GetSubscription(ctx, subscriptionID)
	if err != nil {
		l.Errorf("paddle GetSubscription %s failed (webhook resync): %v", subscriptionID, err)
		return
	}
	l.applyPaddleSubBestEffort(ctx, sub)
}

// cancelPaddleSubscriptionNow cancels a subscription at Paddle immediately
// (refund/chargeback revocation) and applies the returned entity — best
// effort, all errors logged. The resulting subscription.canceled webhook
// also converges the provider state on its own.
func (l *HandlePaddleWebhookLogic) cancelPaddleSubscriptionNow(ctx context.Context, subscriptionID string) {
	cli := l.svcCtx.PaddleClient
	if cli == nil || subscriptionID == "" {
		return
	}
	sub, err := cli.CancelSubscriptionImmediately(ctx, subscriptionID)
	if err != nil {
		l.Errorf("paddle CancelSubscriptionImmediately %s failed (post-refund): %v", subscriptionID, err)
		return
	}
	l.applyPaddleSubBestEffort(ctx, sub)
}

// applyPaddleSubBestEffort resolves the subscription's user via the customer
// link and writes provider state + merged recompute inside a transaction.
func (l *HandlePaddleWebhookLogic) applyPaddleSubBestEffort(ctx context.Context, sub *paddle.Subscription) {
	if err := l.getTxRunner().RunSerializable(ctx, "", func(tx pgx.Tx) error {
		txRepo := l.getTxRepo(tx)
		userID, err := l.resolveByPaddleCustomer(ctx, txRepo, sub.CustomerID)
		if err != nil {
			return err
		}
		return l.applyPaddleSubscriptionEntity(ctx, txRepo, userID, sub, time.Now(), "")
	}); err != nil {
		l.Errorf("paddle subscription state apply %s failed: %v", sub.ID, err)
	}
}

// resolveByPaddleCustomer maps a Paddle customer id to a user via the stored
// link on the merged subscriptions row.
func (l *HandlePaddleWebhookLogic) resolveByPaddleCustomer(ctx context.Context, repo *repository.Repository, customerID string) (uuid.UUID, error) {
	if customerID == "" {
		return uuid.Nil, errPaddleUserNotFound
	}
	row, err := repo.Billing.GetUserSubscriptionByPaddleCustomerID(ctx, &customerID)
	if err == nil {
		return row.UserID, nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return uuid.Nil, errPaddleUserNotFound
	}
	l.Errorf("Failed to look up subscription by paddle customer %s: %v", customerID, err)
	return uuid.Nil, status.Error(codes.Internal, "failed to load subscription")
}

// resolveTransactionUser maps a transaction to a user (B4).
// Order: paddle_checkouts (transactions the backend created — the recorded
// user is authoritative even when custom_data disagrees), then the stored
// paddle_customer_id link. custom_data.user_id on a transaction we did NOT
// create is ignored — it's attacker-controlled client-side input.
func (l *HandlePaddleWebhookLogic) resolveTransactionUser(ctx context.Context, repo *repository.Repository, txn *paddle.Transaction) (uuid.UUID, bool, error) {
	if txn.ID != "" {
		uid, err := repo.Billing.GetPaddleCheckoutUserID(ctx, txn.ID)
		if err == nil {
			if claimed := txn.UserID(); claimed != "" {
				if cu, perr := uuid.Parse(claimed); perr != nil || cu != uid {
					l.Errorf("SECURITY: paddle txn %s custom_data.user_id=%q does not match recorded checkout owner %s — using recorded owner",
						txn.ID, claimed, uid)
				}
			}
			return uid, true, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			l.Errorf("Failed to look up paddle checkout %s: %v", txn.ID, err)
			return uuid.Nil, false, status.Error(codes.Internal, "failed to load subscription")
		}
		if claimed := txn.UserID(); claimed != "" {
			l.Errorf("SECURITY: paddle txn %s carries custom_data.user_id=%q but is not a backend-created checkout — ignoring custom_data",
				txn.ID, claimed)
		}
	}

	if txn.CustomerID != "" {
		uid, err := l.resolveByPaddleCustomer(ctx, repo, txn.CustomerID)
		if err == nil {
			return uid, false, nil
		}
		if !errors.Is(err, errPaddleUserNotFound) {
			return uuid.Nil, false, err
		}
	}
	return uuid.Nil, false, errPaddleUserNotFound
}

// resolveAdjustmentUser maps an adjustment to a user: the recorded checkout
// first (the transaction may be a first purchase), then the customer link
// (renewal adjustments aren't backend checkouts).
func (l *HandlePaddleWebhookLogic) resolveAdjustmentUser(ctx context.Context, repo *repository.Repository, adj *paddle.Adjustment) (uuid.UUID, error) {
	if adj.TransactionID != "" {
		uid, err := repo.Billing.GetPaddleCheckoutUserID(ctx, adj.TransactionID)
		if err == nil {
			return uid, nil
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			l.Errorf("Failed to look up paddle checkout %s: %v", adj.TransactionID, err)
			return uuid.Nil, status.Error(codes.Internal, "failed to load subscription")
		}
	}
	return l.resolveByPaddleCustomer(ctx, repo, adj.CustomerID)
}

// providerStateLive reports whether a stored provider state still represents
// the current subscription relationship — used by the stale-subscription-id
// guard so events for a superseded sub_ id can't clobber it.
func providerStateLive(s *db.SubscriptionProviderState, now time.Time) bool {
	switch s.Status {
	case "active", "trialing", "past_due":
		return s.CurrentPeriodEnd.Valid && s.CurrentPeriodEnd.Time.After(now)
	case "paused":
		// A paused subscription is still the active relationship — new
		// sub ids arriving while paused are stale, not re-subscriptions.
		return true
	default:
		return false
	}
}

// isStaleEvent reports whether an incoming event predates the provider state
// already applied — deliveries are unordered, so an older occurred_at must
// not roll state back (B2). Events without a timestamp always apply.
func isStaleEvent(state *db.SubscriptionProviderState, occurredAt time.Time) bool {
	if state == nil || occurredAt.IsZero() || !state.LastEventAt.Valid {
		return false
	}
	return occurredAt.Before(state.LastEventAt.Time)
}

// stateOrNil converts a GetSubscriptionProviderState result into a nullable
// state pointer (nil when no row exists yet).
func stateOrNil(s db.SubscriptionProviderState, err error) *db.SubscriptionProviderState {
	if err != nil {
		return nil
	}
	return &s
}

// ts converts a time.Time to pgtype.Timestamptz; zero times are invalid.
func ts(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func mapPaddleStatus(s string) string {
	switch s {
	case "active", "trialing", "past_due", "paused", "canceled":
		return s
	default:
		return "expired"
	}
}
