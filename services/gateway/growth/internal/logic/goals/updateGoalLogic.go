package goals

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

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
	// Map MilestoneInput (gateway type) to MilestoneInput (proto type).
	var milestoneInputs []*client.MilestoneInput
	for _, mi := range req.Milestones {
		milestoneInputs = append(milestoneInputs, &client.MilestoneInput{
			Id:    mi.Id,
			Title: mi.Title,
		})
	}
	rpcResp, err := l.svcCtx.ClientRpc.Goals.UpdateGoal(l.ctx, &clientgoals.UpdateGoalRequest{
		GoalId:          req.Id,
		Title:           req.Title,
		Description:     req.Description,
		Category:        req.Category,
		DueDate:         parseDueDate(req.DueDate),
		RelatedHabitIds: req.RelatedHabitIds,
		Measurement:     req.Measurement,
		StartValue:      req.StartValue,
		CurrentValue:    req.CurrentValue,
		TargetValue:     req.TargetValue,
		Unit:            req.Unit,
		MilestoneTitles: req.MilestoneTitles,
		Milestones:      milestoneInputs,
	})
	if err != nil {
		return nil, err
	}

	return &types.GoalResponse{
		Data: rpcGoalToType(rpcResp.Goal),
	}, nil
}
