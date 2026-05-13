package goals

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteGoalLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteGoalLogic {
	return &DeleteGoalLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteGoalLogic) DeleteGoal(req *types.GoalRequest) (resp *types.EmptyResponse, err error) {
	_, err = l.svcCtx.GoalsRpc.DeleteGoal(l.ctx, &clientgoals.DeleteGoalRequest{
		GoalId: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.EmptyResponse{}, nil
}
