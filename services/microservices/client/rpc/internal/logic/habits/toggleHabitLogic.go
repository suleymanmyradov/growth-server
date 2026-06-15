package habitslogic

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type ToggleHabitLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewToggleHabitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ToggleHabitLogic {
	return &ToggleHabitLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ToggleHabitLogic) ToggleHabit(in *client.ToggleHabitRequest) (*client.ToggleHabitResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ToggleHabitLogic.ToggleHabit")
	defer span.End()
	habitID, err := uuid.Parse(in.HabitId)
	if err != nil {
		l.Errorf("Invalid habit ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid habit id")
	}

	// Fetch habit first to get the owner user ID for RLS context.
	preHabit, err := l.svcCtx.Repo.Habits.GetHabitByID(ctx, habitID)
	if err != nil {
		l.Errorf("Failed to get habit: %v", err)
		return nil, status.Error(codes.Internal, "failed to get habit")
	}

	var resultHabit db.GetHabitRow
	err = l.svcCtx.TxRunner.Run(ctx, preHabit.UserID.String(), func(tx pgx.Tx) error {
		txRepo := l.svcCtx.WithTx(tx)

		habit, err := txRepo.Habits.ToggleHabit(ctx, habitID)
		if err != nil {
			return fmt.Errorf("toggle habit: %w", err)
		}
		resultHabit = habit

		// Log activity if habit was marked completed
		if habit.Completed {
			description := fmt.Sprintf("Completed habit: %s (streak: %d)", habit.Name, habit.Streak)
			_, err := txRepo.Activities.CreateActivity(ctx, db.CreateActivityParams{
				Type:    "habit_completed",
				Title:       fmt.Sprintf("Completed %s", habit.Name),
				Description: &description,
				Metadata:    json.RawMessage("{}"),
				UserID:      habit.UserID,
			})
			if err != nil {
				return fmt.Errorf("create activity: %w", err)
			}
		}
		return nil
	})
	if err != nil {
		l.Errorf("Failed to toggle habit in tx: %v", err)
		return nil, status.Error(codes.Internal, "failed to toggle habit in tx")
	}

	return &client.ToggleHabitResponse{
		Habit: habitToProto(resultHabit),
	}, nil
}
