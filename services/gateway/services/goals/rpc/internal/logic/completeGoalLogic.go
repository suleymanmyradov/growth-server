package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/goals"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CompleteGoalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCompleteGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CompleteGoalLogic {
	return &CompleteGoalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CompleteGoalLogic) CompleteGoal(in *goals.CompleteGoalRequest) (*goals.CompleteGoalResponse, error) {
	// todo: add your logic here and delete this line

	return &goals.CompleteGoalResponse{}, nil
}
