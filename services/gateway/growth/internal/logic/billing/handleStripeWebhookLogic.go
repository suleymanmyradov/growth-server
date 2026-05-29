package billing

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientbilling "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/billingservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type HandleStripeWebhookLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewHandleStripeWebhookLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HandleStripeWebhookLogic {
	return &HandleStripeWebhookLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *HandleStripeWebhookLogic) HandleStripeWebhook(req *types.StripeWebhookRequest) (resp *types.StripeWebhookResponse, err error) {
	rpcResp, err := l.svcCtx.BillingRpc.HandleStripeWebhook(l.ctx, &clientbilling.HandleStripeWebhookRequest{
		EventType:   req.EventType,
		PayloadJson: req.PayloadJson,
	})
	if err != nil {
		return nil, err
	}

	return &types.StripeWebhookResponse{
		Processed: rpcResp.Processed,
	}, nil
}
