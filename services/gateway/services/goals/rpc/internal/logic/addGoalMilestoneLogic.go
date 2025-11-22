package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/goals"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type AddGoalMilestoneLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewAddGoalMilestoneLogic(ctx context.Context, svcCtx *svc.ServiceContext) *AddGoalMilestoneLogic {
	return &AddGoalMilestoneLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Milestone operations
func (l *AddGoalMilestoneLogic) AddGoalMilestone(in *goals.AddGoalMilestoneRequest) (*goals.AddGoalMilestoneResponse, error) {
	// todo: add your logic here and delete this line

	return &goals.AddGoalMilestoneResponse{}, nil
}
