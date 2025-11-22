package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/habits/rpc/habits"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/habits/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetHabitCalendarLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetHabitCalendarLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetHabitCalendarLogic {
	return &GetHabitCalendarLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetHabitCalendarLogic) GetHabitCalendar(in *habits.GetHabitCalendarRequest) (*habits.GetHabitCalendarResponse, error) {
	// todo: add your logic here and delete this line

	return &habits.GetHabitCalendarResponse{}, nil
}
