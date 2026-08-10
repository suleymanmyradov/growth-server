package goals

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"

	"github.com/zeromicro/go-zero/core/logx"
)

type LogGoalValueLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewLogGoalValueLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogGoalValueLogic {
	return &LogGoalValueLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *LogGoalValueLogic) LogGoalValue(req *types.LogGoalValueRequest) (resp *types.GoalResponse, err error) {
	rpcResp, err := l.svcCtx.ClientRpc.Goals.LogGoalValue(l.ctx, &clientgoals.LogGoalValueRequest{
		GoalId: req.Id,
		Value:  req.Value,
	})
	if err != nil {
		return nil, err
	}

	return &types.GoalResponse{
		Data: rpcGoalToType(rpcResp.Goal),
	}, nil
}
