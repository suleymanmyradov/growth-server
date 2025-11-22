package logic

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/services/habits/rpc/habits"
	"github.com/suleymanmyradov/growth-server/services/gateway/services/habits/rpc/internal/svc"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetHabitStatsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetHabitStatsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetHabitStatsLogic {
	return &GetHabitStatsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// Analytics
func (l *GetHabitStatsLogic) GetHabitStats(in *habits.GetHabitStatsRequest) (*habits.GetHabitStatsResponse, error) {
	// todo: add your logic here and delete this line

	return &habits.GetHabitStatsResponse{}, nil
}
