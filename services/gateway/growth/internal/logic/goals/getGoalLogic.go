package goals

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGoalLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGoalLogic {
	return &GetGoalLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetGoalLogic) GetGoal(req *types.GoalRequest) (resp *types.GoalResponse, err error) {
	rpcResp, err := l.svcCtx.GoalsRpc.GetGoal(l.ctx, &clientgoals.GetGoalRequest{
		GoalId: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.GoalResponse{
		Data: types.Goal{
			Id:          rpcResp.Goal.Id,
			Title:       rpcResp.Goal.Title,
			Description: rpcResp.Goal.Description,
			Category:    rpcResp.Goal.Category,
			DueDate:     formatTime(rpcResp.Goal.DueDate),
			Progress:    int(rpcResp.Goal.Progress),
			Completed:   rpcResp.Goal.Completed,
			UserId:      rpcResp.Goal.UserId,
			CreatedAt:   formatTime(rpcResp.Goal.CreatedAt),
			UpdatedAt:   formatTime(rpcResp.Goal.UpdatedAt),
		},
	}, nil
}
