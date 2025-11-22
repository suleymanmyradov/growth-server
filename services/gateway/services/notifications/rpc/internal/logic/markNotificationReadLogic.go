package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/notifications"

	"github.com/zeromicro/go-zero/core/logx"
)

type MarkNotificationReadLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewMarkNotificationReadLogic(ctx context.Context, svcCtx *svc.ServiceContext) *MarkNotificationReadLogic {
	return &MarkNotificationReadLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Read status
func (l *MarkNotificationReadLogic) MarkNotificationRead(in *notifications.MarkNotificationReadRequest) (*notifications.MarkNotificationReadResponse, error) {
	// todo: add your logic here and delete this line

	return &notifications.MarkNotificationReadResponse{}, nil
}
