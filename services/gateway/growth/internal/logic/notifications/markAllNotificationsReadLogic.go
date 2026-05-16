// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package notifications

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	notificationsClient "github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/notificationsClient"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type MarkAllNotificationsReadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMarkAllNotificationsReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkAllNotificationsReadLogic {
	return &MarkAllNotificationsReadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MarkAllNotificationsReadLogic) MarkAllNotificationsRead() (resp *types.EmptyResponse, err error) {
	_, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return &types.EmptyResponse{}, nil
	}

	_, err = l.svcCtx.NotificationsRpc.MarkAllNotificationsRead(l.ctx, &notificationsClient.MarkAllNotificationsReadRequest{
		UserId: "",
	})
	if err != nil {
		return nil, err
	}

	return &types.EmptyResponse{}, nil
}
