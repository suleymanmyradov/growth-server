package billingservicelogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "CreateCustomerPortalSessionLogic.CreateCustomerPortalSession")
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

	// Get subscription first — the stored provider customer IDs decide which
	// portal to open based on the stored provider customer ID.
	sub, err := l.svcCtx.Repo.Billing.GetUserSubscription(ctx, userID)
	if err != nil {
		l.Errorf("Failed to get subscription: %v", err)
		return nil, status.Error(codes.NotFound, "subscription not found")
	}

	if !l.svcCtx.Config.Billing.Paddle.Enabled || sub.PaddleCustomerID == nil {
		// No Paddle billing (or the user has no Paddle customer yet) — the
		// frontend treats an empty URL as "nothing to manage".
		return &client.CreateCustomerPortalSessionResponse{}, nil
	}
	if l.svcCtx.PaddleClient == nil {
		l.Errorf("Paddle client not configured")
		return nil, status.Error(codes.Internal, "paddle not configured")
	}
	var subIDs []string
	if sub.PaddleSubscriptionID != nil {
		subIDs = []string{*sub.PaddleSubscriptionID}
	}
	sess, err := l.svcCtx.PaddleClient.CreatePortalSession(ctx, *sub.PaddleCustomerID, subIDs)
	if err != nil {
		l.Errorf("Failed to create Paddle portal session: %v", err)
		return nil, status.Error(codes.Internal, "failed to create portal session")
	}
	return &client.CreateCustomerPortalSessionResponse{
		PortalUrl: sess.URLs.General.Overview,
	}, nil
}
