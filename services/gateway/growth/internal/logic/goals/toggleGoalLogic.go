package goals

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"

	"github.com/zeromicro/go-zero/core/logx"
)

type ToggleGoalLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewToggleGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ToggleGoalLogic {
	return &ToggleGoalLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ToggleGoalLogic) ToggleGoal(req *types.GoalRequest) (resp *types.GoalResponse, err error) {
	rpcResp, err := l.svcCtx.ClientRpc.Goals.ToggleGoal(l.ctx, &clientgoals.ToggleGoalRequest{
		GoalId: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.GoalResponse{
		Data: rpcGoalToType(rpcResp.Goal),
	}, nil
}
