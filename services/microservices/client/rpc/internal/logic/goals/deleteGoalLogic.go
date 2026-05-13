package goalslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type DeleteGoalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteGoalLogic {
	return &DeleteGoalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteGoalLogic) DeleteGoal(in *client.DeleteGoalRequest) (*client.DeleteGoalResponse, error) {
	goalID, err := uuid.Parse(in.GoalId)
	if err != nil {
		l.Errorf("Invalid goal ID: %v", err)
		return nil, err
	}

	err = l.svcCtx.Repo.Goals.DeleteGoal(l.ctx, goalID)
	if err != nil {
		l.Errorf("Failed to delete goal: %v", err)
		return nil, err
	}

	return &client.DeleteGoalResponse{
		Success: true,
	}, nil
}
