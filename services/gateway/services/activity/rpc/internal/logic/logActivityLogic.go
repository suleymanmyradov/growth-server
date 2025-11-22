package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/activity/rpc/activity"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/activity/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogActivityLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLogActivityLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogActivityLogic {
	return &LogActivityLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LogActivityLogic) LogActivity(in *activity.LogActivityRequest) (*activity.LogActivityResponse, error) {
	// todo: add your logic here and delete this line

	return &activity.LogActivityResponse{}, nil
}
