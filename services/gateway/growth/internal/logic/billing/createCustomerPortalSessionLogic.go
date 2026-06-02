package billing

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientbilling "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/billingservice"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateCustomerPortalSessionLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateCustomerPortalSessionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCustomerPortalSessionLogic {
	return &CreateCustomerPortalSessionLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateCustomerPortalSessionLogic) CreateCustomerPortalSession() (resp *types.CreateCustomerPortalSessionResponse, err error) {
	rpcResp, err := l.svcCtx.BillingRpc.CreateCustomerPortalSession(l.ctx, &clientbilling.CreateCustomerPortalSessionRequest{})
	if err != nil {
		return nil, err
	}

	return &types.CreateCustomerPortalSessionResponse{
		Data: types.CreateCustomerPortalSessionData{
			PortalUrl: rpcResp.PortalUrl,
		},
	}, nil
}
