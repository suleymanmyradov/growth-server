package habits

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clienthabits "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habits"

	"github.com/zeromicro/go-zero/core/logx"
)

type GetHabitLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetHabitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetHabitLogic {
	return &GetHabitLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetHabitLogic) GetHabit(req *types.HabitRequest) (resp *types.HabitResponse, err error) {
	rpcResp, err := l.svcCtx.HabitsRpc.GetHabit(l.ctx, &clienthabits.GetHabitRequest{
		HabitId: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.HabitResponse{
		Data: types.Habit{
			Id:            rpcResp.Habit.Id,
			Name:          rpcResp.Habit.Name,
			Description:   rpcResp.Habit.Description,
			Streak:        int(rpcResp.Habit.Streak),
			Completed:     rpcResp.Habit.Completed,
			Category:      rpcResp.Habit.Category,
			UserId:        rpcResp.Habit.UserId,
			RecentHistory: rpcResp.Habit.RecentHistory,
			CreatedAt:     formatTime(rpcResp.Habit.CreatedAt),
			UpdatedAt:     formatTime(rpcResp.Habit.UpdatedAt),
		},
	}, nil
}
