package goalslogic

import (
	"context"
	"encoding/json"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/events"
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

	// Ownership check: verify the caller owns this goal before toggling.
	// ToggleGoal SQL has no WHERE user_id clause, so without this check any
	// authenticated user could toggle any other user's goal by ID.
	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	existing, err := l.svcCtx.Repo.Goals.GetGoalByID(ctx, goalID)
	if err != nil {
		l.Errorf("Failed to get goal: %v", err)
		return nil, status.Error(codes.NotFound, "goal not found")
	}
	if existing.UserID.String() != p.UserID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}

	goal, err := l.svcCtx.Repo.Goals.ToggleGoal(ctx, goalID)
	if err != nil {
		l.Errorf("Failed to toggle goal: %v", err)
		return nil, status.Error(codes.Internal, "failed to toggle goal")
	}

	// When reactivating a derived-type goal, recompute progress from source
	// rows. ToggleGoal SQL sets progress=0 on reactivation; for manual goals
	// 0 is correct (user explicitly un-completed), and for derived types
	// recompute overwrites it with the true computed value.
	if !goal.Completed && goal.Measurement != MeasurementManual {
		goal, err = recomputeAndPersist(ctx, l.svcCtx, goalID)
		if err != nil {
			l.Errorf("Failed to recompute goal progress after toggle: %v", err)
		}
	}

	habitIDs, err := l.svcCtx.Repo.Goals.ListGoalHabitIDsByGoal(ctx, goalID)
	if err != nil {
		l.Errorf("Failed to list goal-habit links: %v", err)
		return nil, status.Error(codes.Internal, "failed to list goal-habit links")
	}

	var milestones []db.GoalMilestone
	if goal.Measurement == MeasurementMilestone {
		milestones, _ = l.svcCtx.Repo.Goals.ListGoalMilestones(ctx, goalID)
	}

	l.svcCtx.InvalidatePersonalizationContext(ctx, goal.UserID)

	// Log goal_completed activity when the goal is toggled to completed.
	if goal.Completed {
		activityDesc := "Completed goal: " + goal.Title
		if _, aErr := l.svcCtx.Repo.Activities.CreateActivity(ctx, db.CreateActivityParams{
			Type:        "goal_completed",
			Title:       goal.Title,
			Description: &activityDesc,
			Metadata:    json.RawMessage("{}"),
			UserID:      goal.UserID,
		}); aErr != nil {
			l.Errorf("Failed to log goal_completed activity: %v", aErr)
		}
	}

	// Fire-and-forget publish goal_completed event when the goal is toggled
	// to completed status (for analytics/metrics).
	if goal.Completed && l.svcCtx.EventsPub != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			env, err := events.NewEnvelope(events.TypeGoalCompleted, events.GoalCompleted{
				UserID: goal.UserID.String(),
				GoalID: goal.ID.String(),
				Title:  goal.Title,
			})
			if err != nil {
				logx.Errorf("envelope: %v", err)
				return
			}
			if err := l.svcCtx.EventsPub.Publish(ctx, env); err != nil {
				logx.Errorf("publish goal_completed event: %v", err)
			}
		}()
	}

	return &client.ToggleGoalResponse{
		Goal: goalToProto(goal, habitUUIDsToStrings(habitIDs), milestones),
	}, nil
}
