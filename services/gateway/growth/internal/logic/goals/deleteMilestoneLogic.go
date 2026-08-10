package goals

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteMilestoneLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteMilestoneLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteMilestoneLogic {
	return &DeleteMilestoneLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteMilestoneLogic) DeleteMilestone(req *types.MilestoneRequest) (resp *types.DeleteMilestoneResponse, err error) {
	_, err = l.svcCtx.ClientRpc.Goals.DeleteMilestone(l.ctx, &clientgoals.DeleteMilestoneRequest{
		GoalId:      req.Id,
		MilestoneId: req.MilestoneId,
	})
	if err != nil {
		return nil, err
	}

	return &types.DeleteMilestoneResponse{
		Success: true,
	}, nil
}
