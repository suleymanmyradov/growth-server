package billing

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	authservice "github.com/suleymanmyradov/growth-server/services/microservices/auth/rpc/authservice"
	clientbilling "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/billingservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreatePaddleCheckoutLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreatePaddleCheckoutLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePaddleCheckoutLogic {
	return &CreatePaddleCheckoutLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreatePaddleCheckoutLogic) CreatePaddleCheckout(req *types.CreatePaddleCheckoutRequest) (resp *types.CreatePaddleCheckoutResponse, err error) {
	rpcResp, err := l.svcCtx.ClientRpc.BillingService.CreatePaddleCheckout(l.ctx, &clientbilling.CreatePaddleCheckoutRequest{
		PriceId:       req.PriceId,
		CheckoutUrl:   req.CheckoutUrl,
		SuccessUrl:    req.SuccessUrl,
		CancelUrl:     req.CancelUrl,
		CustomerEmail: l.verifiedEmail(),
	})
	if err != nil {
		return nil, err
	}

	return &types.CreatePaddleCheckoutResponse{
		CheckoutUrl:   rpcResp.CheckoutUrl,
		TransactionId: rpcResp.TransactionId,
	}, nil
}

// verifiedEmail returns the caller's verified email from auth so checkout can
// pre-fill it. Best effort: on any failure checkout still works — Paddle just
// asks for the email itself — so errors are logged, not returned.
func (l *CreatePaddleCheckoutLogic) verifiedEmail() string {
	profile, err := l.svcCtx.AuthRpc.GetProfile(l.ctx, &authservice.GetProfileRequest{})
	if err != nil {
		l.Errorf("checkout email prefill: get profile: %v", err)
		return ""
	}
	if profile.User == nil || !profile.User.EmailVerified {
		return ""
	}
	return profile.User.Email
}
