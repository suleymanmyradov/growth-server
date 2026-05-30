package goalslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type ToggleGoalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewToggleGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ToggleGoalLogic {
	return &ToggleGoalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ToggleGoalLogic) ToggleGoal(in *client.ToggleGoalRequest) (*client.ToggleGoalResponse, error) {
	goalID, err := uuid.Parse(in.GoalId)
	if err != nil {
		l.Errorf("Invalid goal ID: %v", err)
		return nil, err
	}

	// Fetch current goal to get version for optimistic locking
	current, err := l.svcCtx.Repo.Goals.GetGoalByID(l.ctx, goalID)
	if err != nil {
		l.Errorf("Failed to fetch goal for toggle: %v", err)
		return nil, err
	}

	goal, err := l.svcCtx.Repo.Goals.ToggleGoal(l.ctx, goalID, current.Version)
	if err != nil {
		l.Errorf("Failed to toggle goal: %v", err)
		return nil, err
	}

	return &client.ToggleGoalResponse{
		Goal: goalToProto(goal),
	}, nil
}
