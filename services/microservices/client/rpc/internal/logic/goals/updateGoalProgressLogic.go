package goalslogic

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
)

type UpdateGoalProgressLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewUpdateGoalProgressLogic(ctx context.Context, svcCtx *svc.ServiceContext) *UpdateGoalProgressLogic {
	return &UpdateGoalProgressLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *UpdateGoalProgressLogic) UpdateGoalProgress(in *client.UpdateGoalProgressRequest) (*client.UpdateGoalProgressResponse, error) {
	goalID, err := uuid.Parse(in.GoalId)
	if err != nil {
		l.Errorf("Invalid goal ID: %v", err)
		return nil, err
	}

	params := db.UpdateGoalProgressParams{
		ID:       goalID,
		Progress: sql.NullInt32{Int32: in.Progress, Valid: true},
	}

	goal, err := l.svcCtx.Repo.Goals.UpdateGoalProgress(l.ctx, params)
	if err != nil {
		l.Errorf("Failed to update goal progress: %v", err)
		return nil, err
	}

	return &client.UpdateGoalProgressResponse{
		Goal: goalToProto(goal),
	}, nil
}
