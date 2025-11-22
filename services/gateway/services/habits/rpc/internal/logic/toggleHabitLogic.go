package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/habits/rpc/habits"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/habits/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type ToggleHabitLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewToggleHabitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ToggleHabitLogic {
	return &ToggleHabitLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Habit operations
func (l *ToggleHabitLogic) ToggleHabit(in *habits.ToggleHabitRequest) (*habits.ToggleHabitResponse, error) {
	// todo: add your logic here and delete this line

	return &habits.ToggleHabitResponse{}, nil
}
