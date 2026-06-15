package habitslogic

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateHabitLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateHabitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateHabitLogic {
	return &UpdateHabitLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateHabitLogic) UpdateHabit(in *client.UpdateHabitRequest) (*client.UpdateHabitResponse, error) {
	habitID, err := uuid.Parse(in.HabitId)
	if err != nil {
		l.Errorf("Invalid habit ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid habit id")
	}

	// Fetch current habit to get version for optimistic locking.

	var desc *string
	if in.Description != "" {
		desc = &in.Description
	}

	habit, err := l.svcCtx.Repo.Habits.UpdateHabit(l.ctx, habitID, in.Name, desc, in.Category)
	if err != nil {
		l.Errorf("Failed to update habit: %v", err)
		return nil, status.Error(codes.Internal, "failed to update habit")
	}

	return &client.UpdateHabitResponse{
		Habit: habitToProto(habit),
	}, nil
}
