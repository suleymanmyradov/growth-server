package habitslogic

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UpdateHabitLogic.UpdateHabit")
	defer span.End()
	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	habitID, err := uuid.Parse(in.HabitId)
	if err != nil {
		l.Errorf("Invalid habit ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid habit id")
	}

	// Verify ownership before mutating. GetHabitByID returns NotFound on miss,
	// and we re-check the owner against the principal to prevent IDOR.
	existing, err := l.svcCtx.Repo.Habits.GetHabitByID(ctx, habitID)
	if err != nil {
		l.Errorf("Failed to get habit: %v", err)
		return nil, status.Error(codes.NotFound, "habit not found")
	}
	if existing.UserID.String() != p.UserID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}

	var desc *string
	if in.Description != "" {
		desc = &in.Description
	}

	habit, err := l.svcCtx.Repo.Habits.UpdateHabit(ctx, habitID, in.Name, desc, in.Category)
	if err != nil {
		l.Errorf("Failed to update habit: %v", err)
		return nil, status.Error(codes.Internal, "failed to update habit")
	}

	// Streak is derived from check_ins history, not the stored counter.
	streak, sErr := l.svcCtx.Repo.Habits.GetHabitStreak(ctx, habitID, habit.UserID)
	if sErr != nil {
		l.Errorf("Failed to compute habit streak: %v", sErr)
		streak = 0
	}

	l.svcCtx.InvalidatePersonalizationContext(ctx, habit.UserID)

	return &client.UpdateHabitResponse{
		Habit: habitToProto(habit, streak, nil),
	}, nil
}
