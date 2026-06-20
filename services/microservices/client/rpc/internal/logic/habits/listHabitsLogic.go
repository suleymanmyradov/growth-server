package habitslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListHabitsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListHabitsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListHabitsLogic {
	return &ListHabitsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListHabitsLogic) ListHabits(in *client.ListHabitsRequest) (*client.ListHabitsResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ListHabitsLogic.ListHabits")
	defer span.End()
	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid user id")
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := (in.Page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	habits, err := l.svcCtx.Repo.Habits.ListHabits(ctx, userID, limit, offset)
	if err != nil {
		l.Errorf("Failed to list habits: %v", err)
		return nil, status.Error(codes.Internal, "failed to list habits")
	}

	total, err := l.svcCtx.Repo.Habits.CountHabitsByUser(ctx, userID)
	if err != nil {
		l.Errorf("Failed to count habits: %v", err)
		return nil, status.Error(codes.Internal, "failed to count habits")
	}

	// Streaks are derived from check_ins history (consecutive completed days),
	// not the stored habit.streak counter. Fetch them in one set-based query.
	streakRows, err := l.svcCtx.Repo.Habits.GetHabitStreaks(ctx, userID)
	if err != nil {
		l.Errorf("Failed to compute habit streaks: %v", err)
		return nil, status.Error(codes.Internal, "failed to compute habit streaks")
	}
	streakByHabit := make(map[uuid.UUID]int32, len(streakRows))
	for _, s := range streakRows {
		streakByHabit[s.HabitID] = s.Streak
	}

	// Fetch the last 28 days of completed check-ins (any habit) for the user
	// and build a per-habit boolean history array for the contribution graph.
	historyRows, err := l.svcCtx.Repo.Habits.ListHabitHistory(ctx, userID)
	if err != nil {
		l.Errorf("Failed to list habit history: %v", err)
		return nil, status.Error(codes.Internal, "failed to list habit history")
	}
	// Bucket history rows by habit id once (O(n)) so buildRecentHistory is
	// linear per habit instead of rescanning the full history each time.
	historyByHabit := bucketHabitHistory(historyRows)
	// Use the user's configured timezone so "today" matches the SQL window.
	tz := ""
	if settings, sErr := l.svcCtx.Repo.UserSettings.GetUserSettings(ctx, userID); sErr == nil {
		tz = settings.Timezone
	}
	today := userToday(tz)

	pbHabits := make([]*client.Habit, len(habits))
	for i, h := range habits {
		pbHabits[i] = habitToProto(h, streakByHabit[h.ID], buildRecentHistory(h.ID, today, historyByHabit[h.ID]))
	}

	return &client.ListHabitsResponse{
		Habits: pbHabits,
		Total:  int32(total),
	}, nil
}
