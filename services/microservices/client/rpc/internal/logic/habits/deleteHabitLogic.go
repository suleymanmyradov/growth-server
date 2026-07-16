package habitslogic

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/events"
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
	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	habitID, err := uuid.Parse(in.HabitId)
	if err != nil {
		l.Errorf("Invalid habit ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid habit id")
	}

	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid user id")
	}

	timezone := "UTC"
	if prefs, pErr := l.svcCtx.Repo.UserPreferences.GetUserPreferences(ctx, userID); pErr == nil {
		timezone = prefs.Timezone
	}

	// Verify ownership before deleting. Prevents IDOR: a caller cannot delete
	// another user's habit by supplying its UUID.
	existing, err := l.svcCtx.Repo.Habits.GetHabitByID(ctx, habitID, timezone)
	if err != nil {
		l.Errorf("Failed to get habit: %v", err)
		return nil, status.Error(codes.NotFound, "habit not found")
	}
	if existing.UserID.String() != p.UserID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}

	if err := l.svcCtx.Repo.Habits.DeleteHabit(ctx, habitID); err != nil {
		l.Errorf("Failed to delete habit: %v", err)
		return nil, status.Error(codes.Internal, "failed to delete habit")
	}

	l.svcCtx.InvalidatePersonalizationContext(ctx, existing.UserID)

	// Fire-and-forget publish habit_deleted event so notifications can update
	// its local reminder_state read model (active_habit_count).
	if l.svcCtx.EventsPub != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			env, err := events.NewEnvelope(events.TypeHabitDeleted, events.HabitDeleted{
				UserID:  existing.UserID.String(),
				HabitID: habitID.String(),
			})
			if err != nil {
				logx.Errorf("envelope: %v", err)
				return
			}
			if err := l.svcCtx.EventsPub.Publish(ctx, env); err != nil {
				logx.Errorf("publish habit_deleted event: %v", err)
			}
		}()
	}

	return &client.DeleteHabitResponse{
		Success: true,
	}, nil
}
