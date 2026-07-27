package billingservicelogic

import (
	"context"
	"fmt"
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
// Event handling:
//   - INITIAL_PURCHASE / RENEWAL / UNCANCELLATION: set subscription to 'active'
//     (or 'trialing' if within trial period), update period dates.
//   - CANCELLATION: set cancel_at_period_end = true (user still has access
//     until expiration).
//   - EXPIRATION: set subscription to 'expired' and downgrade to 'free' plan.
//   - ENTITLEMENT_CHANGE / PRODUCT_CHANGE: re-fetch entitlements from
//     RevenueCat and sync the subscription accordingly.
//
// The app_user_id in the webhook is our user UUID (set by Purchases.logIn() on
// the mobile client). If the user has no subscription row yet, a default free
// one is created first, then updated.
//
// Idempotency: each event is processed at most once using the processed_events
// table (consumer = 'revenuecat_webhooks'). If RevenueCat doesn't send an
// event_id, we derive one from the event type + app_user_id + period_start.
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

	if len(payload.Events) == 0 {
		l.Infof("RevenueCat webhook: no events in payload")
		return &client.HandleRevenueCatWebhookResponse{Processed: true}, nil
	}

	processed := 0
	failed := 0
	for _, evt := range payload.Events {
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

	l.Infof("RevenueCat webhook: %d/%d events processed, %d failed", processed, len(payload.Events), failed)

	// If any event had a retryable failure, return a non-2xx response so
	// RevenueCat retries the entire webhook. Events that were already marked
	// processed (including permanent failures) will be skipped on retry
	// (idempotency). Events that had retryable failures will be re-attempted.
	// Do NOT return success (HTTP 200) when there are retryable failures —
	// RevenueCat only retries on non-2xx responses.
	if failed > 0 {
		return nil, status.Error(codes.Internal, fmt.Sprintf("revenuecat webhook: %d/%d events failed", failed, len(payload.Events)))
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

// handleEventWithRepo processes a single RevenueCat webhook event and updates
// the user's subscription using the given (potentially transaction-backed) repo.
func (l *HandleRevenueCatWebhookLogic) handleEventWithRepo(ctx context.Context, repo *repository.Repository, evt revenuecat.WebhookEvent) error {
	userID, err := uuid.Parse(evt.AppUserID)
	if err != nil {
		return fmt.Errorf("invalid app_user_id %q: %w", evt.AppUserID, err)
	}

	// Get or create the user's subscription. GetOrCreateUserSubscription
	// lazy-creates a free subscription if none exists.
	sub, err := repo.Billing.GetOrCreateUserSubscription(ctx, userID)
	if err != nil {
		return fmt.Errorf("get subscription: %w", err)
	}

	// Link the RevenueCat customer ID if not already linked. The app_user_id
	// IS the RevenueCat customer ID after Purchases.logIn().
	if sub.RevenuecatCustomerID == nil || *sub.RevenuecatCustomerID == "" {
		customerID := evt.AppUserID
		if err := repo.Billing.SetRevenueCatCustomerID(ctx, userID, &customerID); err != nil {
			l.Errorf("SetRevenueCatCustomerID failed: %v", err)
			// Non-fatal: continue processing the event.
		}
	}

	switch evt.Type {
	case "INITIAL_PURCHASE", "RENEWAL", "UNCANCELLATION":
		return l.handlePurchaseOrRenewalWithRepo(ctx, repo, userID, sub, evt)
	case "CANCELLATION":
		return l.handleCancellationWithRepo(ctx, repo, userID, sub, evt)
	case "EXPIRATION":
		return l.handleExpirationWithRepo(ctx, repo, userID, sub)
	case "PRODUCT_CHANGE", "ENTITLEMENT_CHANGE":
		return l.handleEntitlementChangeWithRepo(ctx, repo, userID, sub)
	case "NON_RENEWING_PURCHASE":
		// One-time purchase — treat like INITIAL_PURCHASE but without renewal.
		return l.handlePurchaseOrRenewalWithRepo(ctx, repo, userID, sub, evt)
	case "TRANSFER":
		// User transferred purchases between accounts. Re-fetch entitlements.
		return l.handleEntitlementChangeWithRepo(ctx, repo, userID, sub)
	default:
		l.Infof("RevenueCat event type %s not handled, skipping", evt.Type)
		return nil
	}
}

// handlePurchaseOrRenewalWithRepo activates the subscription with the pro plan
// using the given (potentially transaction-backed) repo.
func (l *HandleRevenueCatWebhookLogic) handlePurchaseOrRenewalWithRepo(ctx context.Context, repo *repository.Repository, userID uuid.UUID, sub db.GetUserSubscriptionRow, evt revenuecat.WebhookEvent) error {
	proPlan, err := repo.Billing.GetPlanByCode(ctx, "pro")
	if err != nil {
		return fmt.Errorf("get pro plan: %w", err)
	}

	periodStart := parseTime(evt.PeriodStartAt)
	periodEnd := parseTime(evt.ExpirationAt)
	isTrial := evt.Type == "INITIAL_PURCHASE" && periodEnd.Valid && periodEnd.Time.After(time.Now()) && strings.Contains(strings.ToLower(evt.ProductID), "trial")

	subStatus := "active"
	if isTrial {
		subStatus = "trialing"
	}

	interval := "monthly"
	if strings.Contains(strings.ToLower(evt.ProductID), "annual") || strings.Contains(strings.ToLower(evt.ProductID), "yearly") {
		interval = "annual"
	}
	intervalPtr := &interval

	_, err = repo.Billing.UpsertUserSubscription(ctx, db.UpsertUserSubscriptionParams{
		UserID:             userID,
		PlanID:             proPlan.ID,
		Status:             subStatus,
		BillingInterval:    intervalPtr,
		CurrentPeriodStart: periodStart,
		CurrentPeriodEnd:   periodEnd,
		CancelAtPeriodEnd:  false,
	})
	if err != nil {
		return fmt.Errorf("upsert subscription: %w", err)
	}

	l.Infof("RevenueCat: user %s subscription activated (plan=pro status=%s interval=%s)", userID, subStatus, interval)
	return nil
}

// handleCancellationWithRepo sets cancel_at_period_end = true. The user retains
// access until the current period expires.
func (l *HandleRevenueCatWebhookLogic) handleCancellationWithRepo(ctx context.Context, repo *repository.Repository, userID uuid.UUID, sub db.GetUserSubscriptionRow, evt revenuecat.WebhookEvent) error {
	periodEnd := parseTime(evt.ExpirationAt)
	if !periodEnd.Valid {
		// Use the existing period end if the event doesn't carry one.
		periodEnd = sub.CurrentPeriodEnd
	}

	_, err := repo.Billing.UpsertUserSubscription(ctx, db.UpsertUserSubscriptionParams{
		UserID:            userID,
		PlanID:            sub.PlanID,
		Status:            sub.Status,
		BillingInterval:   sub.BillingInterval,
		CurrentPeriodStart: sub.CurrentPeriodStart,
		CurrentPeriodEnd:   periodEnd,
		CancelAtPeriodEnd:  true,
	})
	if err != nil {
		return fmt.Errorf("upsert subscription (cancel): %w", err)
	}

	l.Infof("RevenueCat: user %s subscription canceled at period end", userID)
	return nil
}

// handleExpirationWithRepo downgrades the user to the free plan with 'expired' status.
func (l *HandleRevenueCatWebhookLogic) handleExpirationWithRepo(ctx context.Context, repo *repository.Repository, userID uuid.UUID, sub db.GetUserSubscriptionRow) error {
	freePlan, err := repo.Billing.GetPlanByCode(ctx, "free")
	if err != nil {
		return fmt.Errorf("get free plan: %w", err)
	}

	_, err = repo.Billing.UpsertUserSubscription(ctx, db.UpsertUserSubscriptionParams{
		UserID:            userID,
		PlanID:            freePlan.ID,
		Status:            "expired",
		BillingInterval:   nil,
		CancelAtPeriodEnd:  false,
	})
	if err != nil {
		return fmt.Errorf("upsert subscription (expire): %w", err)
	}

	l.Infof("RevenueCat: user %s subscription expired, downgraded to free", userID)
	return nil
}

// handleEntitlementChangeWithRepo re-fetches the user's entitlements from
// RevenueCat and syncs the subscription accordingly. This is the most reliable
// way to handle PRODUCT_CHANGE and ENTITLEMENT_CHANGE events.
func (l *HandleRevenueCatWebhookLogic) handleEntitlementChangeWithRepo(ctx context.Context, repo *repository.Repository, userID uuid.UUID, sub db.GetUserSubscriptionRow) error {
	cfg := l.svcCtx.Config.Billing.RevenueCat
	if cfg.APIKey == "" || cfg.ProjectID == "" {
		// Without API credentials, we can't re-fetch. Fall back to keeping
		// the current state — the next webhook with a clear type will fix it.
		l.Infof("RevenueCat: skipping entitlement re-fetch (no API key configured) for user %s", userID)
		return nil
	}

	rcClient := revenuecat.NewClient(cfg.APIKey, cfg.ProjectID, nil)
	entitlements, err := rcClient.GetCustomerEntitlements(ctx, userID.String())
	if err != nil {
		return fmt.Errorf("fetch entitlements: %w", err)
	}

	if revenuecat.HasProEntitlement(entitlements) {
		// User has active pro — ensure subscription reflects this.
		if sub.PlanCode != "pro" || (sub.Status != "active" && sub.Status != "trialing") {
			proPlan, perr := repo.Billing.GetPlanByCode(ctx, "pro")
			if perr != nil {
				return fmt.Errorf("get pro plan: %w", perr)
			}
			_, err = repo.Billing.UpsertUserSubscription(ctx, db.UpsertUserSubscriptionParams{
				UserID:            userID,
				PlanID:            proPlan.ID,
				Status:            "active",
				CancelAtPeriodEnd:  false,
			})
			if err != nil {
				return fmt.Errorf("upsert subscription (entitlement sync): %w", err)
			}
			l.Infof("RevenueCat: user %s upgraded to pro via entitlement sync", userID)
		}
	} else {
		// No active pro entitlement — downgrade to free.
		if sub.PlanCode != "free" {
			freePlan, ferr := repo.Billing.GetPlanByCode(ctx, "free")
			if ferr != nil {
				return fmt.Errorf("get free plan: %w", ferr)
			}
			_, err = repo.Billing.UpsertUserSubscription(ctx, db.UpsertUserSubscriptionParams{
				UserID:            userID,
				PlanID:            freePlan.ID,
				Status:            "expired",
				CancelAtPeriodEnd:  false,
			})
			if err != nil {
				return fmt.Errorf("upsert subscription (entitlement sync downgrade): %w", err)
			}
			l.Infof("RevenueCat: user %s downgraded to free via entitlement sync", userID)
		}
	}

	return nil
}

// deriveEventID builds a stable event ID when RevenueCat doesn't send one.
func deriveEventID(evt revenuecat.WebhookEvent) string {
	raw := fmt.Sprintf("%s:%s:%s:%s", evt.Type, evt.AppUserID, evt.ProductID, evt.PeriodStartAt)
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

// parseTime parses an ISO 8601 datetime string into a pgtype.Timestamptz.
// Returns an invalid Timestamptz for empty strings.
func parseTime(s string) pgtype.Timestamptz {
	s = strings.TrimSpace(s)
	if s == "" {
		return pgtype.Timestamptz{Valid: false}
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return pgtype.Timestamptz{Valid: false}
	}
	return pgtype.Timestamptz{Time: t, Valid: true}
}
