package goals

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateGoalLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateGoalLogic {
	return &UpdateGoalLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateGoalLogic) UpdateGoal(req *types.UpdateGoalRequest) (resp *types.GoalResponse, err error) {
	rpcResp, err := l.svcCtx.GoalsRpc.UpdateGoal(l.ctx, &clientgoals.UpdateGoalRequest{
		GoalId:          req.Id,
		Title:           req.Title,
		Description:     req.Description,
		Category:        req.Category,
		RelatedHabitIds: req.RelatedHabitIds,
	})
	if err != nil {
		return nil, err
	}

	return &types.GoalResponse{
		Data: types.Goal{
			Id:              rpcResp.Goal.Id,
			Title:           rpcResp.Goal.Title,
			Description:     rpcResp.Goal.Description,
			Category:        rpcResp.Goal.Category,
			DueDate:         formatTime(rpcResp.Goal.DueDate),
			Progress:        int(rpcResp.Goal.Progress),
			Completed:       rpcResp.Goal.Completed,
			RelatedHabitIds: nonNilHabitIds(rpcResp.Goal.RelatedHabitIds),
			UserId:          rpcResp.Goal.UserId,
			CreatedAt:       formatTime(rpcResp.Goal.CreatedAt),
			UpdatedAt:       formatTime(rpcResp.Goal.UpdatedAt),
		},
	}, nil
}
