package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/habits/rpc/habits"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/habits/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetHabitLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetHabitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetHabitLogic {
	return &GetHabitLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetHabitLogic) GetHabit(in *habits.GetHabitRequest) (*habits.GetHabitResponse, error) {
	// todo: add your logic here and delete this line

	return &habits.GetHabitResponse{}, nil
}
