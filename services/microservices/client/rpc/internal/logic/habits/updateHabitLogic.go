package habitslogic

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
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
	habitID, err := uuid.Parse(in.HabitId)
	if err != nil {
		l.Errorf("Invalid habit ID: %v", err)
		return nil, err
	}

	params := db.UpdateHabitParams{
		ID:          habitID,
		Name:        in.Name,
		Description: sql.NullString{String: in.Description, Valid: in.Description != ""},
		Category:    in.Category,
	}

	habit, err := l.svcCtx.Repo.Habits.UpdateHabit(l.ctx, params)
	if err != nil {
		l.Errorf("Failed to update habit: %v", err)
		return nil, err
	}

	return &client.UpdateHabitResponse{
		Habit: habitToProto(habit),
	}, nil
}
