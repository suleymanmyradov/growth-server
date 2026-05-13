package habitslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetHabitLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetHabitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetHabitLogic {
	return &GetHabitLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetHabitLogic) GetHabit(in *client.GetHabitRequest) (*client.GetHabitResponse, error) {
	if in == nil || in.HabitId == "" {
		return nil, status.Error(codes.InvalidArgument, "habit ID is required")
	}

	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	habitID, err := uuid.Parse(in.HabitId)
	if err != nil {
		l.Errorf("failed to parse habit ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid habit ID")
	}

	habit, err := l.svcCtx.Repo.Habits.GetHabitByID(l.ctx, habitID)
	if err != nil {
		l.Errorf("failed to get habit: %v", err)
		return nil, status.Error(codes.NotFound, "habit not found")
	}

	if habit.UserID.String() != p.UserID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}

	return &client.GetHabitResponse{
		Habit: habitToProto(habit),
	}, nil
}
