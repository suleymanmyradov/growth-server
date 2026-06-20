package habitslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetHabitLogic.GetHabit")
	defer span.End()
	if in == nil || in.HabitId == "" {
		return nil, status.Error(codes.InvalidArgument, "habit ID is required")
	}

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	habitID, err := uuid.Parse(in.HabitId)
	if err != nil {
		l.Errorf("failed to parse habit ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid habit ID")
	}

	habit, err := l.svcCtx.Repo.Habits.GetHabitByID(ctx, habitID)
	if err != nil {
		l.Errorf("failed to get habit: %v", err)
		return nil, status.Error(codes.NotFound, "habit not found")
	}

	if habit.UserID.String() != p.UserID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}

	// Streak is derived from check_ins history, not the stored counter.
	streak, sErr := l.svcCtx.Repo.Habits.GetHabitStreak(ctx, habitID, habit.UserID)
	if sErr != nil {
		l.Errorf("failed to compute habit streak: %v", sErr)
		streak = 0
	}

	// Build the 28-day recent history the same way ListHabits does so the
	// single-habit view has the same contribution-graph data.
	historyRows, hErr := l.svcCtx.Repo.Habits.ListHabitHistory(ctx, habit.UserID)
	if hErr != nil {
		l.Errorf("failed to list habit history: %v", hErr)
	}
	historyByHabit := bucketHabitHistory(historyRows)
	tz := ""
	if settings, sErr := l.svcCtx.Repo.UserSettings.GetUserSettings(ctx, habit.UserID); sErr == nil {
		tz = settings.Timezone
	}
	today := userToday(tz)
	recentHistory := buildRecentHistory(habitID, today, historyByHabit[habitID])

	return &client.GetHabitResponse{
		Habit: habitToProto(habit, streak, recentHistory),
	}, nil
}
