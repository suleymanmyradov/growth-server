package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/habits/rpc/habits"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/habits/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type CreateHabitLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateHabitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateHabitLogic {
	return &CreateHabitLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateHabitLogic) CreateHabit(in *habits.CreateHabitRequest) (*habits.CreateHabitResponse, error) {
	// todo: add your logic here and delete this line

	return &habits.CreateHabitResponse{}, nil
}
