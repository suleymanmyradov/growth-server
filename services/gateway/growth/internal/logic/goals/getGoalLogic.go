// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package goals

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetGoalLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGoalLogic {
	return &GetGoalLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetGoalLogic) GetGoal(req *types.GoalRequest) (resp *types.GoalResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
