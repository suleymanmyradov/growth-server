package goals

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/validator"
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
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	if !validator.IsNotEmpty(req.Title) {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}
	if !validator.IsNotEmpty(req.Category) {
		return nil, status.Error(codes.InvalidArgument, "category is required")
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
