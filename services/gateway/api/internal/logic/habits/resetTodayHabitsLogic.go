// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package habits

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type ResetTodayHabitsLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewResetTodayHabitsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ResetTodayHabitsLogic {
	return &ResetTodayHabitsLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *ResetTodayHabitsLogic) ResetTodayHabits() (resp *types.EmptyResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
