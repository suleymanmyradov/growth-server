// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package goals

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteGoalLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteGoalLogic {
	return &DeleteGoalLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteGoalLogic) DeleteGoal(req *types.GoalRequest) (resp *types.EmptyResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
