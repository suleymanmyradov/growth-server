// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package habits

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ToggleHabitLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewToggleHabitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ToggleHabitLogic {
	return &ToggleHabitLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ToggleHabitLogic) ToggleHabit(req *types.HabitRequest) (resp *types.HabitResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
