package goalslogic

import (
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/codes"
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ToggleGoalLogic.ToggleGoal")
	defer span.End()
	goalID, err := uuid.Parse(in.GoalId)
	if err != nil {
		l.Errorf("Invalid goal ID: %v", err)
return nil, status.Error(codes.Internal, "invalid goal id")
	}


	goal, err := l.svcCtx.Repo.Goals.ToggleGoal(ctx, goalID)
	if err != nil {
		l.Errorf("Failed to toggle goal: %v", err)
return nil, status.Error(codes.Internal, "failed to toggle goal")
	}

	return &client.ToggleGoalResponse{
		Goal: goalToProto(goal),
	}, nil
}
