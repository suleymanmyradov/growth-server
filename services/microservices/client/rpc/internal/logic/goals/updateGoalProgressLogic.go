package goalslogic

import (
	"context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "UpdateGoalProgressLogic.UpdateGoalProgress")
	defer span.End()
	goalID, err := uuid.Parse(in.GoalId)
	if err != nil {
		l.Errorf("Invalid goal ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid goal id")
	}

	goal, err := l.svcCtx.Repo.Goals.UpdateGoalProgress(ctx, goalID, in.Progress)
	if err != nil {
		l.Errorf("Failed to update goal progress: %v", err)
		return nil, status.Error(codes.Internal, "failed to update goal progress")
	}

	return &client.UpdateGoalProgressResponse{
		Goal: goalToProto(goal),
	}, nil
}
