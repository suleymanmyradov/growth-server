package habits

import (
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clienthabits "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habits"

	"errors"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
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
	_, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, errors.New("unauthenticated")
	}

	rpcResp, err := l.svcCtx.HabitsRpc.CreateHabit(l.ctx, &clienthabits.CreateHabitRequest{
		UserId:      "",
		Name:        req.Name,
		Description: req.Description,
		Category:    req.Category,
	})
	if err != nil {
		return nil, err
	}

	return &types.HabitResponse{
		Data: types.Habit{
			Id:          rpcResp.Habit.Id,
			Name:        rpcResp.Habit.Name,
			Description: rpcResp.Habit.Description,
			Streak:      int(rpcResp.Habit.Streak),
			Completed:   rpcResp.Habit.Completed,
			Category:    rpcResp.Habit.Category,
			UserId:      rpcResp.Habit.UserId,
			CreatedAt:   formatTime(rpcResp.Habit.CreatedAt),
			UpdatedAt:   formatTime(rpcResp.Habit.UpdatedAt),
		},
	}, nil
}
