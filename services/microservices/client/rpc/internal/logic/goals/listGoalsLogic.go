package goalslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ListGoalsLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewListGoalsLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ListGoalsLogic {
	return &ListGoalsLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ListGoalsLogic) ListGoals(in *client.ListGoalsRequest) (*client.ListGoalsResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ListGoalsLogic.ListGoals")
	defer span.End()
	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid user id")
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := (in.Page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	goals, err := l.svcCtx.Repo.Goals.ListGoals(ctx, userID, limit, offset)
	if err != nil {
		l.Errorf("Failed to list goals: %v", err)
		return nil, status.Error(codes.Internal, "failed to list goals")
	}

	total, err := l.svcCtx.Repo.Goals.CountGoalsByUser(ctx, userID)
	if err != nil {
		l.Errorf("Failed to count goals: %v", err)
		return nil, status.Error(codes.Internal, "failed to count goals")
	}

	// Batch-fetch all goal-habit links for this user's goals and group by
	// goal ID so we can populate RelatedHabitIds without N+1 queries.
	linkRows, err := l.svcCtx.Repo.Goals.ListGoalHabitIDs(ctx, userID)
	if err != nil {
		l.Errorf("Failed to list goal-habit links: %v", err)
		return nil, status.Error(codes.Internal, "failed to list goal-habit links")
	}
	habitsByGoal := make(map[uuid.UUID][]string, len(goals))
	for _, r := range linkRows {
		habitsByGoal[r.GoalID] = append(habitsByGoal[r.GoalID], r.HabitID.String())
	}

	pbGoals := make([]*client.Goal, len(goals))
	for i, g := range goals {
		pbGoals[i] = goalToProto(g, habitsByGoal[g.ID])
	}

	return &client.ListGoalsResponse{
		Goals: pbGoals,
		Total: int32(total),
	}, nil
}
