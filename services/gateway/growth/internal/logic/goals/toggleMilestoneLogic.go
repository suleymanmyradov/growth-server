package goals

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"

	"github.com/zeromicro/go-zero/core/logx"
)

type ToggleMilestoneLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewToggleMilestoneLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ToggleMilestoneLogic {
	return &ToggleMilestoneLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ToggleMilestoneLogic) ToggleMilestone(req *types.MilestoneRequest) (resp *types.GoalResponse, err error) {
	rpcResp, err := l.svcCtx.ClientRpc.Goals.ToggleMilestone(l.ctx, &clientgoals.ToggleMilestoneRequest{
		GoalId:      req.Id,
		MilestoneId: req.MilestoneId,
	})
	if err != nil {
		return nil, err
	}

	return &types.GoalResponse{
		Data: rpcGoalToType(rpcResp.Goal),
	}, nil
}
