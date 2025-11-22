package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/activity/rpc/activity"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/activity/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetStreaksLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetStreaksLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetStreaksLogic {
	return &GetStreaksLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetStreaksLogic) GetStreaks(in *activity.GetStreaksRequest) (*activity.GetStreaksResponse, error) {
	// todo: add your logic here and delete this line

	return &activity.GetStreaksResponse{}, nil
}
