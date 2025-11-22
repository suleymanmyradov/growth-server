package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/goals"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/goals/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateGoalProgressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateGoalProgressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateGoalProgressLogic {
	return &UpdateGoalProgressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateGoalProgressLogic) UpdateGoalProgress(in *goals.UpdateGoalProgressRequest) (*goals.UpdateGoalProgressResponse, error) {
	// todo: add your logic here and delete this line

	return &goals.UpdateGoalProgressResponse{}, nil
}
