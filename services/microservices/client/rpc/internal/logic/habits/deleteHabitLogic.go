package habitslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteHabitLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteHabitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteHabitLogic {
	return &DeleteHabitLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteHabitLogic) DeleteHabit(in *client.DeleteHabitRequest) (*client.DeleteHabitResponse, error) {
	habitID, err := uuid.Parse(in.HabitId)
	if err != nil {
		l.Errorf("Invalid habit ID: %v", err)
		return nil, err
	}

	err = l.svcCtx.Repo.Habits.DeleteHabit(l.ctx, habitID)
	if err != nil {
		l.Errorf("Failed to delete habit: %v", err)
		return nil, err
	}

	return &client.DeleteHabitResponse{
		Success: true,
	}, nil
}
