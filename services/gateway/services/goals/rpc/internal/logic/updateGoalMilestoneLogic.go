package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/goals"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateGoalMilestoneLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateGoalMilestoneLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateGoalMilestoneLogic {
	return &UpdateGoalMilestoneLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateGoalMilestoneLogic) UpdateGoalMilestone(in *goals.UpdateGoalMilestoneRequest) (*goals.UpdateGoalMilestoneResponse, error) {
	// todo: add your logic here and delete this line

	return &goals.UpdateGoalMilestoneResponse{}, nil
}
