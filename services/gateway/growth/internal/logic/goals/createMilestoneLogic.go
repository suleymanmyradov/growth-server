package goals

import (
	"context"

	"github.com/suleymanmyradov/growth-server/pkg/validator"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateMilestoneLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateMilestoneLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateMilestoneLogic {
	return &CreateMilestoneLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateMilestoneLogic) CreateMilestone(req *types.CreateMilestoneRequest) (resp *types.GoalResponse, err error) {
	if !validator.IsNotEmpty(req.Title) {
		return nil, status.Error(codes.InvalidArgument, "title is required")
	}

	rpcResp, err := l.svcCtx.ClientRpc.Goals.CreateMilestone(l.ctx, &clientgoals.CreateMilestoneRequest{
		GoalId:    req.Id,
		Title:     req.Title,
		SortOrder: int32(req.SortOrder),
	})
	if err != nil {
		return nil, err
	}

	return &types.GoalResponse{
		Data: rpcGoalToType(rpcResp.Goal),
	}, nil
}
