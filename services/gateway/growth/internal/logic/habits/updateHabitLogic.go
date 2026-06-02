package habits

import (
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	"context"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clienthabits "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habits"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateHabitLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewUpdateHabitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateHabitLogic {
	return &UpdateHabitLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *UpdateHabitLogic) UpdateHabit(req *types.UpdateHabitRequest) (resp *types.HabitResponse, err error) {
	habitID, ok := l.ctx.Value("habitId").(string)
	if !ok {
		l.Error("habitId not found in context")
		return nil, status.Error(codes.Internal, "habitId not found in context")
	}

	rpcResp, err := l.svcCtx.HabitsRpc.UpdateHabit(l.ctx, &clienthabits.UpdateHabitRequest{
		HabitId:     habitID,
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
