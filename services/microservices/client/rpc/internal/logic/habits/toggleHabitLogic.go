package habitslogic

import (
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
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
	habitID, err := uuid.Parse(in.HabitId)
	if err != nil {
		l.Errorf("Invalid habit ID: %v", err)
return nil, status.Error(codes.Internal, "invalid habit id")
	}

	// Fetch habit first to get the owner user ID for RLS context.
	preHabit, err := l.svcCtx.Repo.Habits.GetHabitByID(l.ctx, habitID)
	if err != nil {
		l.Errorf("Failed to get habit: %v", err)
return nil, status.Error(codes.Internal, "failed to get habit")
	}

	var resultHabit db.Habit
	err = l.svcCtx.TxRunner.Run(l.ctx, preHabit.UserID.String(), func(tx pgx.Tx) error {
		txRepo := l.svcCtx.WithTx(tx)

		habit, err := txRepo.Habits.ToggleHabit(l.ctx, habitID, preHabit.Version)
		if err != nil {
			return fmt.Errorf("toggle habit: %w", err)
		}
		resultHabit = habit

		// Log activity if habit was marked completed
		if habit.Completed {
			description := fmt.Sprintf("Completed habit: %s (streak: %d)", habit.Name, habit.Streak)
			_, err := txRepo.Activities.CreateActivity(l.ctx, db.CreateActivityParams{
				ItemType:    "habit_completed",
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
