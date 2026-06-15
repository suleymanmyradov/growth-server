package billingservicelogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateCheckoutSessionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateCheckoutSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCheckoutSessionLogic {
	return &CreateCheckoutSessionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateCheckoutSessionLogic) CreateCheckoutSession(in *client.CreateCheckoutSessionRequest) (*client.CreateCheckoutSessionResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "CreateCheckoutSessionLogic.CreateCheckoutSession")
	defer span.End()
	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	if l.svcCtx.Authz != nil {
		if err := l.svcCtx.Authz.CheckPrincipal(ctx); err != nil {
			return nil, err
		}
	}

	billingMode := l.svcCtx.Config.Billing.Mode
	if billingMode == "" {
		billingMode = "fake_door"
	}

	// In fake_door mode, return empty checkout URL (frontend handles the dialog)
	if billingMode == "fake_door" || billingMode == "disabled" {
		return &client.CreateCheckoutSessionResponse{
			CheckoutUrl: "",
			SessionId:   "",
		}, nil
	}

	// Stripe mode: create a real checkout session
	if billingMode == "stripe_test" || billingMode == "stripe_live" {
		return l.createStripeCheckoutSession(userID, p.Username, in)
	}

	return &client.CreateCheckoutSessionResponse{
		CheckoutUrl: "",
		SessionId:   "",
	}, nil
}

func (l *CreateCheckoutSessionLogic) createStripeCheckoutSession(userID uuid.UUID, username string, in *client.CreateCheckoutSessionRequest) (*client.CreateCheckoutSessionResponse, error) {
	if l.svcCtx.StripeClient == nil {
		l.Errorf("Stripe client not configured")
		return nil, status.Error(codes.Internal, "stripe not configured")
	}

	// Get the plan to find the Stripe price ID
	plan, err := l.svcCtx.Repo.Billing.GetPlanByCode(l.ctx, in.PlanCode)
	if err != nil {
		l.Errorf("Failed to get plan: %v", err)
		return nil, status.Error(codes.NotFound, "plan not found")
	}

	var priceID string
	if in.BillingInterval == "annual" && plan.StripeAnnualPriceID != nil {
		priceID = *plan.StripeAnnualPriceID
	} else if plan.StripeMonthlyPriceID != nil {
		priceID = *plan.StripeMonthlyPriceID
	} else {
		return nil, status.Error(codes.Internal, "stripe price not configured for plan")
	}

	// Get or create subscription for Stripe customer ID
	sub, err := l.svcCtx.Repo.Billing.GetOrCreateUserSubscription(l.ctx, userID)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to get subscription")
	}

	var customerID string
	if sub.StripeCustomerID != nil {
		customerID = *sub.StripeCustomerID
	} else {
		// Create Stripe customer
		customerID, err = l.svcCtx.StripeClient.CreateCustomer(l.ctx, userID.String(), username)
		if err != nil {
			l.Errorf("Failed to create Stripe customer: %v", err)
			return nil, status.Error(codes.Internal, "failed to create customer")
		}
		// Persist the new Stripe customer ID immediately
		freePlan, planErr := l.svcCtx.Repo.Billing.GetPlanByCode(l.ctx, "free")
		if planErr != nil {
			l.Errorf("Failed to get free plan: %v", planErr)
		} else {
			_, upsertErr := l.svcCtx.Repo.Billing.UpsertUserSubscription(l.ctx, db.UpsertUserSubscriptionParams{
				UserID:           userID,
				PlanID:           freePlan.ID,
				Status:           sub.Status,
				StripeCustomerID: &customerID,
			})
			if upsertErr != nil {
				l.Errorf("Failed to persist stripe_customer_id: %v", upsertErr)
			}
		}
	}

	checkoutURL, sessionID, err := l.svcCtx.StripeClient.CreateCheckoutSession(
		l.ctx, customerID, priceID, l.svcCtx.Config.Billing.FrontendURL,
	)
	if err != nil {
		l.Errorf("Failed to create Stripe checkout session: %v", err)
		return nil, status.Error(codes.Internal, "failed to create checkout session")
	}

	return &client.CreateCheckoutSessionResponse{
		CheckoutUrl: checkoutURL,
		SessionId:   sessionID,
	}, nil
}
