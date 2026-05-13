package goalslogic

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateGoalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateGoalLogic {
	return &UpdateGoalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateGoalLogic) UpdateGoal(in *client.UpdateGoalRequest) (*client.UpdateGoalResponse, error) {
	goalID, err := uuid.Parse(in.GoalId)
	if err != nil {
		l.Errorf("Invalid goal ID: %v", err)
		return nil, err
	}

	params := db.UpdateGoalParams{
		ID:          goalID,
		Title:       in.Title,
		Description: sql.NullString{String: in.Description, Valid: in.Description != ""},
		Category:    in.Category,
		DueDate:     sql.NullTime{Time: time.Unix(in.DueDate, 0), Valid: in.DueDate > 0},
	}

	goal, err := l.svcCtx.Repo.Goals.UpdateGoal(l.ctx, params)
	if err != nil {
		l.Errorf("Failed to update goal: %v", err)
		return nil, err
	}

	return &client.UpdateGoalResponse{
		Goal: goalToProto(goal),
	}, nil
}
