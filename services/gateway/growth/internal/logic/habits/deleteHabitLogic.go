package habits

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clienthabits "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habits"

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
	_, err = l.svcCtx.HabitsRpc.DeleteHabit(l.ctx, &clienthabits.DeleteHabitRequest{
		HabitId: req.Id,
	})
	if err != nil {
		return nil, err
	}

	return &types.EmptyResponse{}, nil
}
