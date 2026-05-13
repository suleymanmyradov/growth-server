package habitslogic

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
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
		return nil, err
	}

	habit, err := l.svcCtx.Repo.Habits.ToggleHabit(l.ctx, habitID)
	if err != nil {
		l.Errorf("Failed to toggle habit: %v", err)
		return nil, err
	}

	// Log activity if habit was marked completed
	if habit.Completed.Bool {
		_, err := l.svcCtx.Repo.Activities.CreateActivity(l.ctx, db.CreateActivityParams{
			ItemType:    "habit_completed",
			Title:       fmt.Sprintf("Completed %s", habit.Name),
			Description: sql.NullString{String: fmt.Sprintf("Completed habit: %s (streak: %d)", habit.Name, habit.Streak.Int32), Valid: true},
			Metadata:    pqtype.NullRawMessage{},
			UserID:      habit.UserID,
		})
		if err != nil {
			l.Errorf("Failed to create activity: %v", err)
		}
	}

	return &client.ToggleHabitResponse{
		Habit: habitToProto(habit),
	}, nil
}
