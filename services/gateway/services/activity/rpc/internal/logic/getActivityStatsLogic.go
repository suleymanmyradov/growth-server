package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/activity/rpc/activity"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/activity/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetActivityStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetActivityStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetActivityStatsLogic {
	return &GetActivityStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Analytics
func (l *GetActivityStatsLogic) GetActivityStats(in *activity.GetActivityStatsRequest) (*activity.GetActivityStatsResponse, error) {
	// todo: add your logic here and delete this line

	return &activity.GetActivityStatsResponse{}, nil
}
