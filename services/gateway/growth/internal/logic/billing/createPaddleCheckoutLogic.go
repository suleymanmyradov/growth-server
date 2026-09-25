package billing

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
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
		PriceId:     req.PriceId,
		CheckoutUrl: req.CheckoutUrl,
		SuccessUrl:  req.SuccessUrl,
		CancelUrl:   req.CancelUrl,
	})
	if err != nil {
		return nil, err
	}

	return &types.CreatePaddleCheckoutResponse{
		CheckoutUrl:   rpcResp.CheckoutUrl,
		TransactionId: rpcResp.TransactionId,
	}, nil
}
