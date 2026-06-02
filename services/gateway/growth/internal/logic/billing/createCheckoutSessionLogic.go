package billing

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientbilling "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/billingservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateCheckoutSessionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateCheckoutSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCheckoutSessionLogic {
	return &CreateCheckoutSessionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateCheckoutSessionLogic) CreateCheckoutSession(req *types.CreateCheckoutSessionRequest) (resp *types.CreateCheckoutSessionResponse, err error) {
	rpcResp, err := l.svcCtx.BillingRpc.CreateCheckoutSession(l.ctx, &clientbilling.CreateCheckoutSessionRequest{
		PlanCode:        req.PlanCode,
		BillingInterval: req.BillingInterval,
	})
	if err != nil {
		return nil, err
	}

	return &types.CreateCheckoutSessionResponse{
		Data: types.CreateCheckoutSessionData{
			CheckoutUrl: rpcResp.CheckoutUrl,
			SessionId:   rpcResp.SessionId,
		},
	}, nil
}
