package billingservicelogic

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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

// stripeSubscriptionData represents the data.object from Stripe subscription events.
type stripeSubscriptionData struct {
	Object stripeSubscription `json:"object"`
}

type stripeSubscription struct {
	ID                     string `json:"id"`
	Customer               string `json:"customer"`
	Status                 string `json:"status"`
	CurrentPeriodStart     int64  `json:"current_period_start"`
	CurrentPeriodEnd       int64  `json:"current_period_end"`
	TrialEnd               int64  `json:"trial_end"`
	CancelAtPeriodEnd      bool   `json:"cancel_at_period_end"`
	Items                  struct {
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

func (l *HandleStripeWebhookLogic) HandleStripeWebhook(in *client.HandleStripeWebhookRequest) (*client.HandleStripeWebhookResponse, error) {
	// The gateway verifies the Stripe signature and forwards:
	//   EventType   = the verified Stripe event type (e.g. "checkout.session.completed")
	//   PayloadJson = the Stripe event's "data" object (e.g. {"object":{...}})
	// Use the verified EventType for dispatch; pass PayloadJson directly to handlers
	// since each handler already expects the raw data object.
	switch in.EventType {
	case "checkout.session.completed":
		return l.handleCheckoutCompleted(json.RawMessage(in.PayloadJson))
	case "customer.subscription.created", "customer.subscription.updated":
		return l.handleSubscriptionUpdated(json.RawMessage(in.PayloadJson))
	case "customer.subscription.deleted":
		return l.handleSubscriptionDeleted(json.RawMessage(in.PayloadJson))
	case "invoice.payment_failed":
		return l.handlePaymentFailed(json.RawMessage(in.PayloadJson))
	default:
		l.Infof("Unhandled webhook event type: %s", in.EventType)
		return &client.HandleStripeWebhookResponse{Processed: true}, nil
	}
}

func (l *HandleStripeWebhookLogic) handleCheckoutCompleted(data json.RawMessage) (*client.HandleStripeWebhookResponse, error) {
	var checkout stripeCheckoutData
	if err := json.Unmarshal(data, &checkout); err != nil {
		l.Errorf("Failed to parse checkout data: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid checkout data")
	}

	// Find user by Stripe customer ID
	existingSub, err := l.svcCtx.Repo.Billing.GetUserSubscriptionByStripeCustomerID(l.ctx, sql.NullString{String: checkout.Object.Customer, Valid: true})
	if err != nil {
		l.Errorf("Failed to find subscription by Stripe customer ID: %v", err)
		return nil, status.Error(codes.NotFound, "subscription not found")
	}

	// Record checkout completion event for audit trail
	_, eventErr := l.svcCtx.Repo.Billing.CreateUpgradeEvent(l.ctx, db.CreateUpgradeEventParams{
		UserID:    existingSub.UserID,
		EventType: "checkout_completed",
		Surface:   "stripe_webhook",
		PlanCode:  sql.NullString{String: "pro", Valid: true},
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
				Status:               "active",
				StripeCustomerID:     sql.NullString{String: checkout.Object.Customer, Valid: true},
				StripeSubscriptionID: sql.NullString{String: checkout.Object.Subscription, Valid: true},
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
	existingSub, err := l.svcCtx.Repo.Billing.GetUserSubscriptionByStripeCustomerID(l.ctx, sql.NullString{String: sub.Customer, Valid: true})
	if err != nil {
		l.Errorf("Failed to find subscription by Stripe customer ID: %v", err)
		return nil, status.Error(codes.NotFound, "subscription not found")
	}

	// Determine plan based on Stripe price ID
	planCode := "pro" // Default to pro for paid subscriptions
	plan, err := l.svcCtx.Repo.Billing.GetPlanByCode(l.ctx, planCode)
	if err != nil {
		l.Errorf("Failed to get plan: %v", err)
		return nil, status.Error(codes.NotFound, "plan not found")
	}

	var periodStart, periodEnd sql.NullTime
	if sub.CurrentPeriodStart > 0 {
		periodStart = sql.NullTime{Time: time.Unix(sub.CurrentPeriodStart, 0), Valid: true}
	}
	if sub.CurrentPeriodEnd > 0 {
		periodEnd = sql.NullTime{Time: time.Unix(sub.CurrentPeriodEnd, 0), Valid: true}
	}

	var trialEndTime sql.NullTime
	if sub.TrialEnd > 0 {
		trialEndTime = sql.NullTime{Time: time.Unix(sub.TrialEnd, 0), Valid: true}
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

	_, err = l.svcCtx.Repo.Billing.UpsertUserSubscription(l.ctx, db.UpsertUserSubscriptionParams{
		UserID:                 existingSub.UserID,
		PlanID:                plan.ID,
		Status:                localStatus,
		BillingInterval:       stringToNullString(billingInterval),
		CurrentPeriodStart:    periodStart,
		CurrentPeriodEnd:      periodEnd,
		TrialEnd:              trialEndTime,
		CancelAtPeriodEnd:     sub.CancelAtPeriodEnd,
		StripeCustomerID:     sql.NullString{String: sub.Customer, Valid: true},
		StripeSubscriptionID:  sql.NullString{String: sub.ID, Valid: true},
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

	existingSub, err := l.svcCtx.Repo.Billing.GetUserSubscriptionByStripeCustomerID(l.ctx, sql.NullString{String: subData.Object.Customer, Valid: true})
	if err != nil {
		l.Errorf("Failed to find subscription by Stripe customer ID: %v", err)
		return nil, status.Error(codes.NotFound, "subscription not found")
	}

	// Downgrade to free plan
	freePlan, err := l.svcCtx.Repo.Billing.GetPlanByCode(l.ctx, "free")
	if err != nil {
		l.Errorf("Failed to get free plan: %v", err)
		return nil, status.Error(codes.NotFound, "free plan not found")
	}

	_, err = l.svcCtx.Repo.Billing.UpsertUserSubscription(l.ctx, db.UpsertUserSubscriptionParams{
		UserID:           existingSub.UserID,
		PlanID:           freePlan.ID,
		Status:           "canceled",
		CancelAtPeriodEnd: false,
		StripeCustomerID: sql.NullString{String: subData.Object.Customer, Valid: true},
		StripeSubscriptionID: sql.NullString{String: subData.Object.ID, Valid: true},
	})
	if err != nil {
		l.Errorf("Failed to downgrade subscription: %v", err)
		return nil, status.Error(codes.Internal, "failed to update subscription")
	}

	return &client.HandleStripeWebhookResponse{Processed: true}, nil
}

func (l *HandleStripeWebhookLogic) handlePaymentFailed(data json.RawMessage) (*client.HandleStripeWebhookResponse, error) {
	var subData stripeSubscriptionData
	if err := json.Unmarshal(data, &subData); err != nil {
		l.Errorf("Failed to parse subscription data: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid subscription data")
	}

	existingSub, err := l.svcCtx.Repo.Billing.GetUserSubscriptionByStripeCustomerID(l.ctx, sql.NullString{String: subData.Object.Customer, Valid: true})
	if err != nil {
		l.Errorf("Failed to find subscription by Stripe customer ID: %v", err)
		return nil, status.Error(codes.NotFound, "subscription not found")
	}

	// Update status to past_due
	proPlan, err := l.svcCtx.Repo.Billing.GetPlanByCode(l.ctx, "pro")
	if err != nil {
		l.Errorf("Failed to get pro plan: %v", err)
		return nil, status.Error(codes.NotFound, "plan not found")
	}

	_, err = l.svcCtx.Repo.Billing.UpsertUserSubscription(l.ctx, db.UpsertUserSubscriptionParams{
		UserID:              existingSub.UserID,
		PlanID:              proPlan.ID,
		Status:              "past_due",
		StripeCustomerID:    sql.NullString{String: subData.Object.Customer, Valid: true},
		StripeSubscriptionID: sql.NullString{String: subData.Object.ID, Valid: true},
	})
	if err != nil {
		l.Errorf("Failed to update subscription to past_due: %v", err)
		return nil, status.Error(codes.Internal, "failed to update subscription")
	}

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
