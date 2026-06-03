package billingservicelogic

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// stripeEventTopLevel is used to extract the event ID from the raw Stripe payload.
type stripeEventTopLevel struct {
	ID   string          `json:"id"`
	Type string          `json:"type"`
	Data json.RawMessage `json:"data"`
}

// stripeSubscriptionData represents the data.object from Stripe subscription events.
type stripeSubscriptionData struct {
	Object stripeSubscription `json:"object"`
}

type stripeSubscription struct {
	ID                 string `json:"id"`
	Customer           string `json:"customer"`
	Status             string `json:"status"`
	CurrentPeriodStart int64  `json:"current_period_start"`
	CurrentPeriodEnd   int64  `json:"current_period_end"`
	TrialEnd           int64  `json:"trial_end"`
	CancelAtPeriodEnd  bool   `json:"cancel_at_period_end"`
	Items              struct {
		Data []stripeSubscriptionItem `json:"data"`
	} `json:"items"`
}

type stripeSubscriptionItem struct {
	Price stripePrice `json:"price"`
}

type stripePrice struct {
	ID        string `json:"id"`
	Recurring struct {
		Interval string `json:"interval"`
	} `json:"recurring"`
}

// stripeCheckoutData represents the data.object from Stripe checkout events.
type stripeCheckoutData struct {
	Object stripeCheckoutSession `json:"object"`
}

type stripeCheckoutSession struct {
	ID           string `json:"id"`
	Customer     string `json:"customer"`
	Subscription string `json:"subscription"`
}

// stripeInvoiceData represents the data.object from Stripe invoice events.
type stripeInvoiceData struct {
	Object stripeInvoice `json:"object"`
}

type stripeInvoice struct {
	ID           string `json:"id"`
	Customer     string `json:"customer"`
	Subscription string `json:"subscription"`
	Status       string `json:"status"`
}

// stripeDisputeData represents the data.object from Stripe dispute events.
type stripeDisputeData struct {
	Object stripeDispute `json:"object"`
}

type stripeDispute struct {
	ID        string `json:"id"`
	Charge    string `json:"charge"`
	Amount    int64  `json:"amount"`
	Currency  string `json:"currency"`
	Status    string `json:"status"`
	Reason    string `json:"reason"`
	PaymentIntent string `json:"payment_intent"`
}

type HandleStripeWebhookLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewHandleStripeWebhookLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HandleStripeWebhookLogic {
	return &HandleStripeWebhookLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *HandleStripeWebhookLogic) HandleStripeWebhook(in *client.HandleStripeWebhookRequest) (*client.HandleStripeWebhookResponse, error) {
	// Idempotency: skip duplicate events using the Stripe event ID.
	if in.StripeEventId != "" {
		processed, err := l.svcCtx.Repo.Billing.IsStripeEventProcessed(l.ctx, in.StripeEventId)
		if err != nil {
			l.Errorf("idempotency check failed: %v", err)
			return nil, status.Error(codes.Internal, "idempotency check failed")
		}
		if processed {
			l.Infof("duplicate webhook skipped: %s", in.StripeEventId)
			return &client.HandleStripeWebhookResponse{Processed: true}, nil
		}
	}

	var result *client.HandleStripeWebhookResponse
	var handleErr error

	switch in.EventType {
	case "checkout.session.completed":
		result, handleErr = l.handleCheckoutCompleted(json.RawMessage(in.PayloadJson))
	case "customer.subscription.created", "customer.subscription.updated":
		result, handleErr = l.handleSubscriptionUpdated(json.RawMessage(in.PayloadJson))
	case "customer.subscription.deleted":
		result, handleErr = l.handleSubscriptionDeleted(json.RawMessage(in.PayloadJson))
	case "invoice.payment_failed":
		result, handleErr = l.handlePaymentFailed(json.RawMessage(in.PayloadJson))
	case "charge.dispute.created":
		result, handleErr = l.handleDisputeCreated(json.RawMessage(in.PayloadJson))
	default:
		l.Infof("Unhandled webhook event type: %s", in.EventType)
		result = &client.HandleStripeWebhookResponse{Processed: true}
	}

	// Mark as processed only on success to allow retries on transient failures.
	if handleErr == nil && in.StripeEventId != "" {
		if markErr := l.svcCtx.Repo.Billing.MarkStripeEventProcessed(l.ctx, in.StripeEventId, in.EventType); markErr != nil {
			l.Errorf("failed to mark stripe event processed: %v", markErr)
			// Non-fatal: the business logic succeeded.
		}
	}

	return result, handleErr
}

func (l *HandleStripeWebhookLogic) handleCheckoutCompleted(data json.RawMessage) (*client.HandleStripeWebhookResponse, error) {
	var checkout stripeCheckoutData
	if err := json.Unmarshal(data, &checkout); err != nil {
		l.Errorf("Failed to parse checkout data: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid checkout data")
	}

	// Find user by Stripe customer ID
	customerID := checkout.Object.Customer
	existingSub, err := l.svcCtx.Repo.Billing.GetUserSubscriptionByStripeCustomerID(l.ctx, &customerID)
	if err != nil {
		l.Errorf("Failed to find subscription by Stripe customer ID: %v", err)
		return nil, status.Error(codes.NotFound, "subscription not found")
	}

	// Record checkout completion event for audit trail
	planCode := "pro"
	_, eventErr := l.svcCtx.Repo.Billing.CreateUpgradeEvent(l.ctx, db.CreateUpgradeEventParams{
		UserID:    existingSub.UserID,
		EventType: "checkout_completed",
		Surface:   "stripe_webhook",
		PlanCode:  &planCode,
	})
	if eventErr != nil {
		l.Errorf("Failed to record checkout completion event: %v", eventErr)
		// Non-fatal: continue processing
	}

	// Update subscription with the new stripe_subscription_id if available
	if checkout.Object.Subscription != "" {
		proPlan, planErr := l.svcCtx.Repo.Billing.GetPlanByCode(l.ctx, "pro")
		if planErr == nil {
			_, upsertErr := l.svcCtx.Repo.Billing.UpsertUserSubscription(l.ctx, db.UpsertUserSubscriptionParams{
				UserID:               existingSub.UserID,
				PlanID:               proPlan.ID,
				Status:               db.SubscriptionStatusTypeActive,
				StripeCustomerID:     &checkout.Object.Customer,
				StripeSubscriptionID: &checkout.Object.Subscription,
			})
			if upsertErr != nil {
				l.Errorf("Failed to update subscription after checkout: %v", upsertErr)
			}
		}
	}

	l.Infof("Checkout completed: session=%s customer=%s", checkout.Object.ID, checkout.Object.Customer)
	return &client.HandleStripeWebhookResponse{Processed: true}, nil
}

func (l *HandleStripeWebhookLogic) handleSubscriptionUpdated(data json.RawMessage) (*client.HandleStripeWebhookResponse, error) {
	var subData stripeSubscriptionData
	if err := json.Unmarshal(data, &subData); err != nil {
		l.Errorf("Failed to parse subscription data: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid subscription data")
	}

	sub := subData.Object
	localStatus := mapStripeStatus(sub.Status)

	// Find user by Stripe customer ID - query the subscription table
	customerID := sub.Customer
	existingSub, err := l.svcCtx.Repo.Billing.GetUserSubscriptionByStripeCustomerID(l.ctx, &customerID)
	if err != nil {
		l.Errorf("Failed to find subscription by Stripe customer ID: %v", err)
		return nil, status.Error(codes.NotFound, "subscription not found")
	}

	// Guard against stale webhooks: if the DB has a different subscription ID,
	// the user has already re-subscribed and we should not overwrite the new one.
	if existingSub.StripeSubscriptionID != nil && *existingSub.StripeSubscriptionID != sub.ID {
		l.Infof("Stale subscription update ignored: db_sub=%s webhook_sub=%s customer=%s",
			*existingSub.StripeSubscriptionID, sub.ID, customerID)
		return &client.HandleStripeWebhookResponse{Processed: true}, nil
	}

	// Determine plan based on Stripe price ID
	planCode := "pro" // Default to pro for paid subscriptions
	plan, err := l.svcCtx.Repo.Billing.GetPlanByCode(l.ctx, planCode)
	if err != nil {
		l.Errorf("Failed to get plan: %v", err)
		return nil, status.Error(codes.NotFound, "plan not found")
	}

	var periodStart, periodEnd pgtype.Timestamptz
	if sub.CurrentPeriodStart > 0 {
		periodStart = pgtype.Timestamptz{Time: time.Unix(sub.CurrentPeriodStart, 0), Valid: true}
	}
	if sub.CurrentPeriodEnd > 0 {
		periodEnd = pgtype.Timestamptz{Time: time.Unix(sub.CurrentPeriodEnd, 0), Valid: true}
	}

	var trialEndTime pgtype.Timestamptz
	if sub.TrialEnd > 0 {
		trialEndTime = pgtype.Timestamptz{Time: time.Unix(sub.TrialEnd, 0), Valid: true}
	}

	// Determine billing interval from Stripe subscription items
	billingInterval := "monthly"
	if len(sub.Items.Data) > 0 && sub.Items.Data[0].Price.Recurring.Interval != "" {
		switch sub.Items.Data[0].Price.Recurring.Interval {
		case "year":
			billingInterval = "annual"
		case "month":
			billingInterval = "monthly"
		default:
			billingInterval = sub.Items.Data[0].Price.Recurring.Interval
		}
	}

	var billingIntervalPtr *db.BillingIntervalType
	if billingInterval != "" {
		bi := db.BillingIntervalType(billingInterval)
		billingIntervalPtr = &bi
	}

	_, err = l.svcCtx.Repo.Billing.UpsertUserSubscription(l.ctx, db.UpsertUserSubscriptionParams{
		UserID:               existingSub.UserID,
		PlanID:               plan.ID,
		Status:               db.SubscriptionStatusType(localStatus),
		BillingInterval:      billingIntervalPtr,
		CurrentPeriodStart:   periodStart,
		CurrentPeriodEnd:     periodEnd,
		TrialEnd:             trialEndTime,
		CancelAtPeriodEnd:    sub.CancelAtPeriodEnd,
		StripeCustomerID:     &sub.Customer,
		StripeSubscriptionID: &sub.ID,
	})
	if err != nil {
		l.Errorf("Failed to upsert subscription: %v", err)
		return nil, status.Error(codes.Internal, "failed to update subscription")
	}

	return &client.HandleStripeWebhookResponse{Processed: true}, nil
}

func (l *HandleStripeWebhookLogic) handleSubscriptionDeleted(data json.RawMessage) (*client.HandleStripeWebhookResponse, error) {
	var subData stripeSubscriptionData
	if err := json.Unmarshal(data, &subData); err != nil {
		l.Errorf("Failed to parse subscription data: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid subscription data")
	}

	customerID := subData.Object.Customer
	existingSub, err := l.svcCtx.Repo.Billing.GetUserSubscriptionByStripeCustomerID(l.ctx, &customerID)
	if err != nil {
		l.Errorf("Failed to find subscription by Stripe customer ID: %v", err)
		return nil, status.Error(codes.NotFound, "subscription not found")
	}

	// Guard against stale webhooks: if the DB has a different subscription ID,
	// the user has already re-subscribed and we should not overwrite the new one.
	if existingSub.StripeSubscriptionID != nil && *existingSub.StripeSubscriptionID != subData.Object.ID {
		l.Infof("Stale subscription deletion ignored: db_sub=%s webhook_sub=%s customer=%s",
			*existingSub.StripeSubscriptionID, subData.Object.ID, customerID)
		return &client.HandleStripeWebhookResponse{Processed: true}, nil
	}

	// Downgrade to free plan
	freePlan, err := l.svcCtx.Repo.Billing.GetPlanByCode(l.ctx, "free")
	if err != nil {
		l.Errorf("Failed to get free plan: %v", err)
		return nil, status.Error(codes.NotFound, "free plan not found")
	}

	_, err = l.svcCtx.Repo.Billing.UpsertUserSubscription(l.ctx, db.UpsertUserSubscriptionParams{
		UserID:               existingSub.UserID,
		PlanID:               freePlan.ID,
		Status:               db.SubscriptionStatusTypeCanceled,
		CancelAtPeriodEnd:    false,
		StripeCustomerID:     &subData.Object.Customer,
		StripeSubscriptionID: &subData.Object.ID,
	})
	if err != nil {
		l.Errorf("Failed to downgrade subscription: %v", err)
		return nil, status.Error(codes.Internal, "failed to update subscription")
	}

	return &client.HandleStripeWebhookResponse{Processed: true}, nil
}

func (l *HandleStripeWebhookLogic) handlePaymentFailed(data json.RawMessage) (*client.HandleStripeWebhookResponse, error) {
	var invoiceData stripeInvoiceData
	if err := json.Unmarshal(data, &invoiceData); err != nil {
		l.Errorf("Failed to parse invoice data: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid invoice data")
	}

	invoice := invoiceData.Object
	customerID := invoice.Customer
	existingSub, err := l.svcCtx.Repo.Billing.GetUserSubscriptionByStripeCustomerID(l.ctx, &customerID)
	if err != nil {
		l.Errorf("Failed to find subscription by Stripe customer ID: %v", err)
		return nil, status.Error(codes.NotFound, "subscription not found")
	}

	// Update status to past_due; keep the current plan so grace-period logic applies.
	_, err = l.svcCtx.Repo.Billing.UpsertUserSubscription(l.ctx, db.UpsertUserSubscriptionParams{
		UserID:               existingSub.UserID,
		PlanID:               existingSub.PlanID,
		Status:               db.SubscriptionStatusTypePastDue,
		StripeCustomerID:     &invoice.Customer,
		StripeSubscriptionID: &invoice.Subscription,
	})
	if err != nil {
		l.Errorf("Failed to update subscription to past_due: %v", err)
		return nil, status.Error(codes.Internal, "failed to update subscription")
	}

	l.Infof("Payment failed: invoice=%s customer=%s subscription=%s", invoice.ID, invoice.Customer, invoice.Subscription)
	return &client.HandleStripeWebhookResponse{Processed: true}, nil
}

func (l *HandleStripeWebhookLogic) handleDisputeCreated(data json.RawMessage) (*client.HandleStripeWebhookResponse, error) {
	var disputeData stripeDisputeData
	if err := json.Unmarshal(data, &disputeData); err != nil {
		l.Errorf("Failed to parse dispute data: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid dispute data")
	}

	dispute := disputeData.Object

	// Log dispute for audit and ops visibility. In production, integrate with
	// your support/alerts pipeline (e.g., email ops, create ticket).
	l.Errorf("STRIPE DISPUTE: id=%s charge=%s amount=%d %s reason=%s status=%s",
		dispute.ID, dispute.Charge, dispute.Amount, dispute.Currency,
		dispute.Reason, dispute.Status,
	)

	// Freeze the subscription by downgrading to free until the dispute is resolved.
	// This prevents continued service access during a contested charge.
	// When the dispute is won (charge.dispute.closed with status=won), the customer
	// can re-subscribe and the checkout webhook will restore their plan.
	//
	// Note: We do not have a direct customer mapping from the dispute payload.
	// In a full implementation, resolve the charge -> payment_intent -> customer
	// via Stripe API or maintain a charge lookup table. For now, log aggressively.

	return &client.HandleStripeWebhookResponse{Processed: true}, nil
}

func mapStripeStatus(stripeStatus string) string {
	switch stripeStatus {
	case "trialing":
		return "trialing"
	case "active":
		return "active"
	case "past_due":
		return "past_due"
	case "canceled":
		return "canceled"
	case "incomplete_expired":
		return "expired"
	default:
		return "expired"
	}
}
