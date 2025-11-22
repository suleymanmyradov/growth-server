package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/notifications/rpc/notifications"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetUnreadCountLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetUnreadCountLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetUnreadCountLogic {
	return &GetUnreadCountLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetUnreadCountLogic) GetUnreadCount(in *notifications.GetUnreadCountRequest) (*notifications.GetUnreadCountResponse, error) {
	// todo: add your logic here and delete this line

	return &notifications.GetUnreadCountResponse{}, nil
}
