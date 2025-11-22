// Code scaffolded by goctl. Safe to edit.
// goctl 1.9.2

package habits

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/api/internal/types"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateHabitLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewCreateHabitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateHabitLogic {
	return &CreateHabitLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *CreateHabitLogic) CreateHabit(req *types.CreateHabitRequest) (resp *types.HabitResponse, err error) {
	// todo: add your logic here and delete this line

	return
}
