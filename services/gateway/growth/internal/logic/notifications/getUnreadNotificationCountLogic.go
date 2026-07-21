// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package notifications

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	notificationsClient "github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/notificationsClient"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetUnreadNotificationCountLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetUnreadNotificationCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUnreadNotificationCountLogic {
	return &GetUnreadNotificationCountLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetUnreadNotificationCountLogic) GetUnreadNotificationCount() (resp *types.UnreadNotificationCountResponse, err error) {
	if _, ok := principal.PrincipalFrom(l.ctx); !ok {
		return &types.UnreadNotificationCountResponse{Count: 0}, nil
	}

	rpcResp, err := l.svcCtx.NotificationsRpc.GetUnreadCount(l.ctx, &notificationsClient.GetUnreadCountRequest{})
	if err != nil {
		return nil, err
	}

	return &types.UnreadNotificationCountResponse{
		Count: rpcResp.Count,
	}, nil
}
