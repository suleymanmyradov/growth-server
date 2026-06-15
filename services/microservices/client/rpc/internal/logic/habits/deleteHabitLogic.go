package habitslogic

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "DeleteHabitLogic.DeleteHabit")
	defer span.End()
	habitID, err := uuid.Parse(in.HabitId)
	if err != nil {
		l.Errorf("Invalid habit ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid habit id")
	}

	err = l.svcCtx.Repo.Habits.DeleteHabit(ctx, habitID)
	if err != nil {
		l.Errorf("Failed to delete habit: %v", err)
		return nil, status.Error(codes.Internal, "failed to delete habit")
	}

	return &client.DeleteHabitResponse{
		Success: true,
	}, nil
}
