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

type HandleRevenueCatWebhookLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewHandleRevenueCatWebhookLogic(ctx context.Context, svcCtx *svc.ServiceContext) *HandleRevenueCatWebhookLogic {
	return &HandleRevenueCatWebhookLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

// HandleRevenueCatWebhook forwards the raw body + Authorization header to the
// client service's HandleRevenueCatWebhook RPC. The client service verifies
// the webhook signature and processes the events. The gateway is just a
// transport layer (same pattern as the Stripe webhook).
func (l *HandleRevenueCatWebhookLogic) HandleRevenueCatWebhook(rawBody []byte, authorization string) (*types.RevenueCatWebhookResponse, error) {
	rpcResp, err := l.svcCtx.ClientRpc.BillingService.HandleRevenueCatWebhook(l.ctx, &clientbilling.HandleRevenueCatWebhookRequest{
		RawBody:       rawBody,
		Authorization: authorization,
	})
	if err != nil {
		return nil, err
	}

	return &types.RevenueCatWebhookResponse{
		Processed: rpcResp.GetProcessed(),
	}, nil
}
