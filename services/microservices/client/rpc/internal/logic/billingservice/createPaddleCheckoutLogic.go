package billingservicelogic

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/paddle"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/jackc/pgx/v5"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreatePaddleCheckoutLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePaddleCheckoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePaddleCheckoutLogic {
	return &CreatePaddleCheckoutLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// CreatePaddleCheckout creates a Paddle transaction for the authenticated user
// and returns the hosted checkout URL + transaction ID. The transaction's
// custom_data.user_id lets Paddle webhooks map the payment back to the user.
func (l *CreatePaddleCheckoutLogic) CreatePaddleCheckout(in *client.CreatePaddleCheckoutRequest) (*client.CreatePaddleCheckoutResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "CreatePaddleCheckoutLogic.CreatePaddleCheckout")
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

	priceID := strings.TrimSpace(in.PriceId)
	if !strings.HasPrefix(priceID, "pri_") {
		return nil, status.Error(codes.InvalidArgument, "priceId must be a Paddle price ID (pri_...)")
	}
	if !l.svcCtx.Config.Billing.Paddle.Enabled || l.svcCtx.PaddleClient == nil {
		return nil, status.Error(codes.FailedPrecondition, "paddle billing is not enabled")
	}

	// Reuse the stored Paddle customer when the user already paid via Paddle so
	// checkout pre-fills their details and stays attached to the same ctm_ id.
	var customerID string
	sub, err := l.svcCtx.Repo.Billing.GetUserSubscription(ctx, userID)
	switch {
	case err == nil && sub.PaddleCustomerID != nil:
		customerID = *sub.PaddleCustomerID
	case err != nil && !errors.Is(err, pgx.ErrNoRows):
		return nil, status.Errorf(codes.Internal, "lookup subscription: %v", err)
	}

	txn, err := l.svcCtx.PaddleClient.CreateTransaction(ctx, paddle.CreateTransactionParams{
		Items:       []paddle.TransactionItemParam{{PriceID: priceID, Quantity: 1}},
		CustomerID:  customerID,
		UserID:      userID.String(),
		CheckoutURL: in.CheckoutUrl,
		SuccessURL:  in.SuccessUrl,
		CancelURL:   in.CancelUrl,
	})
	if err != nil {
		l.Errorf("Paddle CreateTransaction failed for user %s: %v", userID, err)
		return nil, status.Error(codes.Internal, "failed to create paddle checkout")
	}

	l.Infof("Paddle checkout created: txn=%s user=%s customer=%s", txn.ID, userID, customerID)
	return &client.CreatePaddleCheckoutResponse{
		CheckoutUrl:   txn.CheckoutURL(),
		TransactionId: txn.ID,
	}, nil
}
