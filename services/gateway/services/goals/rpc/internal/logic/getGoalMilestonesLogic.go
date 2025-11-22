package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/goals"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGoalMilestonesLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGoalMilestonesLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGoalMilestonesLogic {
	return &GetGoalMilestonesLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetGoalMilestonesLogic) GetGoalMilestones(in *goals.GetGoalMilestonesRequest) (*goals.GetGoalMilestonesResponse, error) {
	// todo: add your logic here and delete this line

	return &goals.GetGoalMilestonesResponse{}, nil
}
