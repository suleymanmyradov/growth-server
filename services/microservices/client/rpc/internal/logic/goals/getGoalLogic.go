package goalslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetGoalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetGoalLogic {
	return &GetGoalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetGoalLogic) GetGoal(in *client.GetGoalRequest) (*client.GetGoalResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetGoalLogic.GetGoal")
	defer span.End()
	if in == nil || in.GoalId == "" {
		return nil, status.Error(codes.InvalidArgument, "goal ID is required")
	}

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	goalID, err := uuid.Parse(in.GoalId)
	if err != nil {
		l.Errorf("failed to parse goal ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid goal ID")
	}

	goal, err := l.svcCtx.Repo.Goals.GetGoalByID(ctx, goalID)
	if err != nil {
		l.Errorf("failed to get goal: %v", err)
		return nil, status.Error(codes.NotFound, "goal not found")
	}

	if goal.UserID.String() != p.UserID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}

	habitIDs, err := l.svcCtx.Repo.Goals.ListGoalHabitIDsByGoal(ctx, goalID)
	if err != nil {
		l.Errorf("Failed to list goal-habit links: %v", err)
		return nil, status.Error(codes.Internal, "failed to list goal-habit links")
	}

	return &client.GetGoalResponse{
		Goal: goalToProto(goal, habitUUIDsToStrings(habitIDs)),
	}, nil
}
