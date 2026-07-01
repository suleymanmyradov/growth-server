package goalslogic

import (
	"context"
	"encoding/json"
	"fmt"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
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

	habitIDs, err := l.svcCtx.Repo.Goals.ListGoalHabitIDsByGoal(ctx, goalID)
	if err != nil {
		l.Errorf("Failed to list goal-habit links: %v", err)
		return nil, status.Error(codes.Internal, "failed to list goal-habit links")
	}

	l.svcCtx.InvalidatePersonalizationContext(ctx, goal.UserID)

	// Log activity only when the goal is marked as completed
	if goal.Completed {
		desc := fmt.Sprintf("Completed goal: %s", goal.Title)
		meta, _ := json.Marshal(map[string]string{"goalId": goal.ID.String()})
		if _, err := l.svcCtx.Repo.Activities.CreateActivity(ctx, db.CreateActivityParams{
			Type:        "goal_completed",
			Title:       fmt.Sprintf("Completed %s", goal.Title),
			Description: &desc,
			Metadata:    meta,
			UserID:      goal.UserID,
		}); err != nil {
			l.Errorf("Failed to log goal_completed activity: %v", err)
		}
	}

	return &client.ToggleGoalResponse{
		Goal: goalToProto(goal, habitUUIDsToStrings(habitIDs)),
	}, nil
}
