package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/goals"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGoalStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGoalStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGoalStatsLogic {
	return &GetGoalStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Analytics
func (l *GetGoalStatsLogic) GetGoalStats(in *goals.GetGoalStatsRequest) (*goals.GetGoalStatsResponse, error) {
	// todo: add your logic here and delete this line

	return &goals.GetGoalStatsResponse{}, nil
}
