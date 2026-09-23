// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package billing

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientbilling "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/billingservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type HandlePaddleWebhookLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewHandlePaddleWebhookLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HandlePaddleWebhookLogic {
	return &HandlePaddleWebhookLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// HandlePaddleWebhook forwards the raw body + Paddle-Signature header to the
// client service's HandlePaddleWebhook RPC. The client service verifies the
// webhook signature and processes the events. The gateway is just a transport
// layer (same pattern as the Stripe webhook).
func (l *HandlePaddleWebhookLogic) HandlePaddleWebhook(rawBody []byte, signature string) (*types.PaddleWebhookResponse, error) {
	rpcResp, err := l.svcCtx.ClientRpc.BillingService.HandlePaddleWebhook(l.ctx, &clientbilling.HandlePaddleWebhookRequest{
		RawBody:   rawBody,
		Signature: signature,
	})
	if err != nil {
		return nil, err
	}

	return &types.PaddleWebhookResponse{
		Processed: rpcResp.GetProcessed(),
	}, nil
}
