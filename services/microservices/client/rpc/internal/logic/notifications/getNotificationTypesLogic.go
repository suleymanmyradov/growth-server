package notificationslogic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetNotificationTypesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetNotificationTypesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetNotificationTypesLogic {
	return &GetNotificationTypesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetNotificationTypesLogic) GetNotificationTypes(in *client.GetNotificationTypesRequest) (*client.GetNotificationTypesResponse, error) {
	l.Logger.Infof("Getting notification types")

	return &client.GetNotificationTypesResponse{
		Types: []*client.NotificationType{},
	}, nil
}
