// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package notifications

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	notificationsClient "github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/notificationsClient"

	"github.com/zeromicro/go-zero/core/logx"
)

type UnsubscribeEmailLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUnsubscribeEmailLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UnsubscribeEmailLogic {
	return &UnsubscribeEmailLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UnsubscribeEmailLogic) UnsubscribeEmail(req *types.UnsubscribeEmailRequest) (resp *types.EmptyResponse, err error) {
	// No principal check: this endpoint is public and token-authenticated. The
	// RPC verifies the HMAC token itself; s2s still guards the call.
	if _, err := l.svcCtx.NotificationsRpc.UnsubscribeEmail(l.ctx, &notificationsClient.UnsubscribeEmailRequest{
		Token: req.Token,
	}); err != nil {
		return nil, err
	}
	return &types.EmptyResponse{}, nil
}
