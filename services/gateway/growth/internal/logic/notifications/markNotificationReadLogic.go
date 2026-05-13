// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package notifications

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientnotifications "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/notifications"

	"github.com/zeromicro/go-zero/core/logx"
)

type MarkNotificationReadLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewMarkNotificationReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkNotificationReadLogic {
	return &MarkNotificationReadLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *MarkNotificationReadLogic) MarkNotificationRead(req *types.NotificationRequest) (resp *types.EmptyResponse, err error) {
	_, err = l.svcCtx.ClientRpc.MarkNotificationRead(l.ctx, &clientnotifications.MarkNotificationReadRequest{
		NotificationId: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.EmptyResponse{}, nil
}
