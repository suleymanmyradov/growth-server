package billingservicelogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateCustomerPortalSessionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateCustomerPortalSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCustomerPortalSessionLogic {
	return &CreateCustomerPortalSessionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateCustomerPortalSessionLogic) CreateCustomerPortalSession(in *client.CreateCustomerPortalSessionRequest) (*client.CreateCustomerPortalSessionResponse, error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	billingMode := l.svcCtx.Config.Billing.Mode
	if billingMode != "stripe_test" && billingMode != "stripe_live" {
		return &client.CreateCustomerPortalSessionResponse{
			PortalUrl: "",
		}, nil
	}

	if l.svcCtx.StripeClient == nil {
		l.Errorf("Stripe client not configured")
		return nil, status.Error(codes.Internal, "stripe not configured")
	}

	// Get subscription to find Stripe customer ID
	sub, err := l.svcCtx.Repo.Billing.GetUserSubscription(l.ctx, userID)
	if err != nil {
		l.Errorf("Failed to get subscription: %v", err)
		return nil, status.Error(codes.NotFound, "subscription not found")
	}

	if sub.StripeCustomerID == nil {
		return nil, status.Error(codes.FailedPrecondition, "no Stripe customer ID")
	}

	portalURL, err := l.svcCtx.StripeClient.CreateCustomerPortalSession(
		l.ctx, *sub.StripeCustomerID, l.svcCtx.Config.Billing.FrontendURL,
	)
	if err != nil {
		l.Errorf("Failed to create Stripe portal session: %v", err)
		return nil, status.Error(codes.Internal, "failed to create portal session")
	}

	return &client.CreateCustomerPortalSessionResponse{
		PortalUrl: portalURL,
	}, nil
}
