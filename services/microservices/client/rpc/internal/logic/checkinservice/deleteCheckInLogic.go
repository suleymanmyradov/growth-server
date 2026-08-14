package checkinservicelogic

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	goalslogic "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/logic/goals"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type DeleteCheckInLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteCheckInLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteCheckInLogic {
	return &DeleteCheckInLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// DeleteCheckIn undoes today's check-in for a habit (the "undo" affordance
// after a one-tap check-in). It deletes the check_ins row for today; the
// streak is derived from check_ins history so it recomputes automatically.
// Linked habit-driven goals are recomputed after the delete.
func (l *DeleteCheckInLogic) DeleteCheckIn(in *client.DeleteCheckInRequest) (*client.DeleteCheckInResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "DeleteCheckInLogic.DeleteCheckIn")
	defer span.End()

	if in.HabitId == "" {
		return nil, status.Error(codes.InvalidArgument, "habitId is required")
	}

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid user id")
	}

	habitID, err := uuid.Parse(in.HabitId)
	if err != nil {
		l.Errorf("Invalid habit ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid habit ID")
	}

	var habit db.GetHabitRow
	var streak int32
	err = l.svcCtx.TxRunner.Run(ctx, userID.String(), func(tx pgx.Tx) error {
		txRepo := l.svcCtx.WithTx(tx)

		timezone := "UTC"
		if prefs, pErr := txRepo.UserPreferences.GetUserPreferences(ctx, userID); pErr == nil {
			timezone = prefs.Timezone
		}

		// Verify the habit exists and belongs to the caller (prevent IDOR).
		habit, err = txRepo.Habits.GetHabitByID(ctx, habitID, timezone)
		if err != nil {
			return status.Error(codes.NotFound, "habit not found")
		}
		if habit.UserID != userID {
			return status.Error(codes.PermissionDenied, "access denied")
		}

		// Delete today's check-in (any status — completed or missed).
		rowsDeleted, dErr := txRepo.CheckIns.DeleteTodayCheckIn(ctx, userID, habitID, timezone)
		if dErr != nil {
			return fmt.Errorf("delete check-in: %w", dErr)
		}
		if rowsDeleted == 0 {
			return status.Error(codes.NotFound, "no check-in to undo for today")
		}

		// Recompute streak from history (today's check-in is gone, so a
		// completed streak day is removed if it was the only source).
		if s, sErr := txRepo.Habits.GetHabitStreak(ctx, habitID, userID, timezone); sErr == nil {
			streak = s
		}

		// Re-fetch the habit so completed=false reflects the undo.
		habit, err = txRepo.Habits.GetHabitByID(ctx, habitID, timezone)
		if err != nil {
			return fmt.Errorf("refetch habit: %w", err)
		}

		// Recompute progress for any habit-driven goals linked to this habit.
		linkedGoalIDs, gErr := txRepo.Goals.ListGoalIDsByHabit(ctx, habitID)
		if gErr != nil {
			return fmt.Errorf("list linked goals: %w", gErr)
		}
		for _, gid := range linkedGoalIDs {
			if _, rErr := goalslogic.RecomputeGoalProgressWithRepo(ctx, txRepo.Goals, txRepo.CheckIns, gid); rErr != nil {
				return fmt.Errorf("recompute goal %s progress: %w", gid, rErr)
			}
		}
		return nil
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
			return nil, st.Err()
		}
		l.Errorf("Failed undo check-in: %v", err)
		return nil, status.Error(codes.Internal, "failed to undo check-in")
	}

	// Invalidate the cached personalization context so coaching reflects the undo.
	l.svcCtx.InvalidatePersonalizationContext(ctx, userID)

	return &client.DeleteCheckInResponse{
		Habit: habitToProto(habit, streak),
	}, nil
}
