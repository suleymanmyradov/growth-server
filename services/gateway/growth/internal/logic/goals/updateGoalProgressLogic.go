package goals

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateGoalProgressLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateGoalProgressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateGoalProgressLogic {
	return &UpdateGoalProgressLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateGoalProgressLogic) UpdateGoalProgress(req *types.UpdateProgressRequest) (resp *types.GoalResponse, err error) {
	rpcResp, err := l.svcCtx.ClientRpc.Goals.UpdateGoalProgress(l.ctx, &clientgoals.UpdateGoalProgressRequest{
		GoalId:   req.Id,
		Progress: int32(req.Progress),
	})
	if err != nil {
		return nil, err
	}

	return &types.GoalResponse{
		Data: rpcGoalToType(rpcResp.Goal),
	}, nil
}
