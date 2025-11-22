package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/goals"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateGoalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateGoalLogic {
	return &CreateGoalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateGoalLogic) CreateGoal(in *goals.CreateGoalRequest) (*goals.CreateGoalResponse, error) {
	// todo: add your logic here and delete this line

	return &goals.CreateGoalResponse{}, nil
}
