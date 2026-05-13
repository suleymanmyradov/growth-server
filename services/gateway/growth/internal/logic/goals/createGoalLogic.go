package goals

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"

	"errors"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type CreateGoalLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateGoalLogic {
	return &CreateGoalLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateGoalLogic) CreateGoal(req *types.CreateGoalRequest) (resp *types.GoalResponse, err error) {
	_, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, errors.New("unauthenticated")
	}

	rpcResp, err := l.svcCtx.GoalsRpc.CreateGoal(l.ctx, &clientgoals.CreateGoalRequest{
		UserId:      "",
		Title:       req.Title,
		Description: req.Description,
		Category:    req.Category,
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
