// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package goals

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateGoalProgressLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateGoalProgressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateGoalProgressLogic {
	return &UpdateGoalProgressLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateGoalProgressLogic) UpdateGoalProgress(req *types.UpdateProgressRequest) (resp *types.GoalResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
