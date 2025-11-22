package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/goals"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGoalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGoalLogic {
	return &GetGoalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetGoalLogic) GetGoal(in *goals.GetGoalRequest) (*goals.GetGoalResponse, error) {
	// todo: add your logic here and delete this line

	return &goals.GetGoalResponse{}, nil
}
