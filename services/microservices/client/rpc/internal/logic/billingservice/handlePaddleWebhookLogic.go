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

// errPaddleUserNotFound marks events we can never map to a user (no
// custom_data.user_id and no existing paddle_customer_id link). Retrying the
// identical payload cannot fix that, so these are marked processed rather
// than burning Paddle's retry budget.
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
// subscription state into the subscriptions table.
//
// Events handled:
//   - subscription.*: convergent upsert of status, period, interval, and
//     cancel_at_period_end (mapped from scheduled_change.action 'cancel' or
//     'pause' — both end paid access at effective_at).
//   - transaction.completed: links paddle IDs + records a checkout_completed
//     upgrade event for the audit trail.
//
// User linkage: checkout stamps custom_data.user_id on the transaction, which
// transaction.completed carries. Paddle does NOT copy it onto the created
// subscription, so subscription.* events resolve via the stored
// paddle_customer_id link instead.
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
			result, handleErr = l.handleTransactionCompleted(ctx, txRepo, event)
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
	return result, nil
}

// handleSubscriptionEvent mirrors a Paddle subscription entity into the user's
// subscription row (convergent upsert — safe for out-of-order and duplicate
// deliveries).
func (l *HandlePaddleWebhookLogic) handleSubscriptionEvent(ctx context.Context, repo *repository.Repository, event *paddle.Envelope) (*client.HandlePaddleWebhookResponse, error) {
	sub, err := event.Subscription()
	if err != nil {
		l.Errorf("Failed to parse paddle subscription data: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid subscription data")
	}

	existingSub, err := l.resolveUserSubscription(ctx, repo, sub.UserID(), sub.CustomerID)
	if err != nil {
		return nil, err
	}

	// Stale-event guard: events for a superseded subscription must not clobber
	// the current one. Only applies while the stored subscription is still
	// live — once it reaches a terminal status, a different incoming
	// subscription id is a legitimate re-subscription.
	if existingSub.PaddleSubscriptionID != nil && *existingSub.PaddleSubscriptionID != sub.ID &&
		isLiveSubscriptionStatus(existingSub.Status) {
		l.Infof("Stale subscription update ignored: db_sub=%s webhook_sub=%s customer=%s",
			*existingSub.PaddleSubscriptionID, sub.ID, sub.CustomerID)
		return &client.HandlePaddleWebhookResponse{Processed: true}, nil
	}

	localStatus := mapPaddleStatus(sub.Status)

	planCode := "pro"
	if localStatus == "canceled" {
		planCode = "free"
	}
	plan, err := repo.Billing.GetPlanByCode(ctx, planCode)
	if err != nil {
		l.Errorf("Failed to get %s plan: %v", planCode, err)
		return nil, status.Error(codes.NotFound, "plan not found")
	}

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

	_, err = repo.Billing.UpsertUserSubscriptionPaddle(ctx, db.UpsertUserSubscriptionPaddleParams{
		UserID:               existingSub.UserID,
		PlanID:               plan.ID,
		Status:               localStatus,
		BillingInterval:      billingInterval,
		CurrentPeriodStart:   periodStart,
		CurrentPeriodEnd:     periodEnd,
		TrialEnd:             trialEnd,
		CancelAtPeriodEnd:    cancelAtPeriodEnd,
		PaddleCustomerID:     &sub.CustomerID,
		PaddleSubscriptionID: &sub.ID,
	})
	if err != nil {
		l.Errorf("Failed to upsert paddle subscription: %v", err)
		return nil, status.Error(codes.Internal, "failed to update subscription")
	}

	l.Infof("Paddle subscription synced: sub=%s customer=%s status=%s",
		sub.ID, sub.CustomerID, localStatus)
	return &client.HandlePaddleWebhookResponse{Processed: true}, nil
}

// handleTransactionCompleted links the Paddle customer/subscription IDs and
// records a checkout_completed upgrade event. The subscription.* events carry
// the full state; this is the audit trail for the payment itself.
func (l *HandlePaddleWebhookLogic) handleTransactionCompleted(ctx context.Context, repo *repository.Repository, event *paddle.Envelope) (*client.HandlePaddleWebhookResponse, error) {
	txn, err := event.Transaction()
	if err != nil {
		l.Errorf("Failed to parse paddle transaction data: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid transaction data")
	}

	existingSub, err := l.resolveUserSubscription(ctx, repo, txn.UserID(), txn.CustomerID)
	if err != nil {
		return nil, err
	}

	_, eventErr := repo.Billing.CreateUpgradeEvent(ctx, db.CreateUpgradeEventParams{
		UserID:    existingSub.UserID,
		EventType: "checkout_completed",
		Surface:   "paddle_webhook",
		Code:      "pro",
		Metadata:  []byte("{}"),
	})
	if eventErr != nil {
		l.Errorf("Failed to record checkout completion event: %v", eventErr)
		// Non-fatal: continue processing
	}

	// Link the Paddle IDs while preserving the current state — the
	// subscription.created event (which carries periods and status) may arrive
	// before OR after this one.
	if txn.SubscriptionID != "" {
		customerID := txn.CustomerID
		_, upsertErr := repo.Billing.UpsertUserSubscriptionPaddle(ctx, db.UpsertUserSubscriptionPaddleParams{
			UserID:               existingSub.UserID,
			PlanID:               existingSub.PlanID,
			Status:               existingSub.Status,
			BillingInterval:      existingSub.BillingInterval,
			CurrentPeriodStart:   existingSub.CurrentPeriodStart,
			CurrentPeriodEnd:     existingSub.CurrentPeriodEnd,
			TrialEnd:             existingSub.TrialEnd,
			CancelAtPeriodEnd:    existingSub.CancelAtPeriodEnd,
			PaddleCustomerID:     &customerID,
			PaddleSubscriptionID: &txn.SubscriptionID,
		})
		if upsertErr != nil {
			l.Errorf("Failed to link paddle subscription: %v", upsertErr)
			return nil, status.Error(codes.Internal, "failed to update subscription")
		}
	}

	l.Infof("Paddle transaction completed: txn=%s customer=%s", txn.ID, txn.CustomerID)
	return &client.HandlePaddleWebhookResponse{Processed: true}, nil
}

// resolveUserSubscription maps a Paddle event to a local subscription row.
// Order: custom_data.user_id (set by our checkout), then the stored
// paddle_customer_id link. Returns errPaddleUserNotFound when neither works.
func (l *HandlePaddleWebhookLogic) resolveUserSubscription(ctx context.Context, repo *repository.Repository, customDataUserID, customerID string) (db.GetUserSubscriptionRow, error) {
	if customDataUserID != "" {
		if userID, err := uuid.Parse(customDataUserID); err == nil {
			sub, err := repo.Billing.GetOrCreateUserSubscription(ctx, userID)
			if err != nil {
				l.Errorf("Failed to get subscription for user %s: %v", userID, err)
				return db.GetUserSubscriptionRow{}, status.Error(codes.Internal, "failed to load subscription")
			}
			return sub, nil
		}
		l.Errorf("Paddle event custom_data.user_id is not a UUID: %q", customDataUserID)
	}

	if customerID != "" {
		row, err := repo.Billing.GetUserSubscriptionByPaddleCustomerID(ctx, &customerID)
		if err == nil {
			return repo.Billing.GetOrCreateUserSubscription(ctx, row.UserID)
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			l.Errorf("Failed to look up subscription by paddle customer %s: %v", customerID, err)
			return db.GetUserSubscriptionRow{}, status.Error(codes.Internal, "failed to load subscription")
		}
	}

	return db.GetUserSubscriptionRow{}, errPaddleUserNotFound
}

// ts converts a time.Time to pgtype.Timestamptz; zero times are invalid.
func ts(t time.Time) pgtype.Timestamptz {
	if t.IsZero() {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func isLiveSubscriptionStatus(s string) bool {
	switch s {
	case "active", "trialing", "past_due", "paused":
		return true
	default:
		return false
	}
}

func mapPaddleStatus(s string) string {
	switch s {
	case "active", "trialing", "past_due", "paused", "canceled":
		return s
	default:
		return "expired"
	}
}
