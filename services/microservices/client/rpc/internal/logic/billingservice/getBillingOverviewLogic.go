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

type GetBillingOverviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetBillingOverviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetBillingOverviewLogic {
	return &GetBillingOverviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetBillingOverviewLogic) GetBillingOverview(in *client.GetBillingOverviewRequest) (*client.GetBillingOverviewResponse, error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// List active plans
	plans, err := l.svcCtx.Repo.Billing.ListActivePlans(l.ctx)
	if err != nil {
		l.Errorf("Failed to list plans: %v", err)
		return nil, status.Error(codes.Internal, "failed to list plans")
	}

	// Get or create user subscription (lazy creation)
	sub, err := l.svcCtx.Repo.Billing.GetUserSubscription(l.ctx, userID)
	if err != nil {
		// If no subscription exists, create a free one automatically
		_, createErr := l.svcCtx.Repo.Billing.CreateDefaultFreeSubscription(l.ctx, userID)
		if createErr != nil {
			l.Errorf("Failed to create default free subscription: %v", createErr)
			return nil, status.Error(codes.Internal, "failed to create subscription")
		}
		// Fetch the newly created subscription
		sub, err = l.svcCtx.Repo.Billing.GetUserSubscription(l.ctx, userID)
		if err != nil {
			l.Errorf("Failed to get subscription after creation: %v", err)
			return nil, status.Error(codes.Internal, "failed to get subscription")
		}
	}

	// Compute entitlements
	entitlements, err := l.svcCtx.Repo.Billing.ComputeEntitlements(l.ctx, sub, userID)
	if err != nil {
		l.Errorf("Failed to compute entitlements: %v", err)
		return nil, status.Error(codes.Internal, "failed to compute entitlements")
	}

	// Convert to proto
	pbPlans := make([]*client.Plan, len(plans))
	for i, plan := range plans {
		pbPlans[i] = planToProto(plan)
	}

	billingMode := "fake_door"
	if l.svcCtx.Config.Billing.StripeSecretKey != "" {
		billingMode = l.svcCtx.Config.Billing.Mode
	}

	return &client.GetBillingOverviewResponse{
		Plans:        pbPlans,
		Subscription: subscriptionToProto(sub),
		Entitlements: entitlementsToProto(entitlements),
		BillingMode:  billingMode,
	}, nil
}
