// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package habits

import (
	"context"

	"gateway/internal/svc"
	"gateway/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteHabitLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewDeleteHabitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteHabitLogic {
	return &DeleteHabitLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *DeleteHabitLogic) DeleteHabit(req *types.HabitRequest) (resp *types.EmptyResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
