package goalslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	commonlogic "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/logic/common"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateGoalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateGoalLogic {
	return &CreateGoalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateGoalLogic) CreateGoal(in *client.CreateGoalRequest) (*client.CreateGoalResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "CreateGoalLogic.CreateGoal")
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

	// Check plan limit enforcement (auto-create free subscription if missing).
	// Users who haven't completed onboarding are exempt — they must always be
	// able to create their initial goal, even if a previous partial onboarding
	// attempt left leftover goals that would otherwise trip the Free plan limit.
	sub, subErr := l.svcCtx.Repo.Billing.GetOrCreateUserSubscription(ctx, userID)
	if subErr == nil {
		entitlements, computeErr := l.svcCtx.Repo.Billing.ComputeEntitlements(ctx, sub, userID)
		if computeErr == nil && !entitlements.CanCreateGoal {
			if !commonlogic.IsOnboardingComplete(ctx, l.svcCtx, userID) {
				l.Infof("CreateGoal: bypassing plan limit for user %s (onboarding not complete)", userID)
			} else {
				st := status.New(codes.FailedPrecondition, "plan limit reached")
				st, _ = st.WithDetails(&client.PlanLimitDetail{
					Limit:          "active_goals",
					UpgradeTrigger: "goal_limit",
				})
				return nil, st.Err()
			}
		}
	}

	// Validate measurement type and type-specific requirements.
	measurement := in.Measurement
	if measurement == "" {
		measurement = MeasurementManual
	}
	if !validMeasurement(measurement) {
		return nil, status.Error(codes.InvalidArgument, "measurement must be one of: binary, numeric, milestone, habit, manual")
	}
	if measurement == MeasurementHabit && len(in.RelatedHabitIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "habit goals require at least one linked habit")
	}
	if measurement == MeasurementNumeric && in.TargetValue == in.StartValue {
		return nil, status.Error(codes.InvalidArgument, "numeric goals require a target value different from start value")
	}

	params := protoToGoalParams(in.Title, in.Description, in.Category, in.DueDate, userID,
		measurement, in.StartValue, in.CurrentValue, in.TargetValue, in.Unit)
	goal, err := l.svcCtx.Repo.Goals.CreateGoal(ctx, params)
	if err != nil {
		l.Errorf("Failed to create goal: %v", err)
		return nil, status.Error(codes.Internal, "failed to create goal")
	}

	// Link habits to the new goal if any were provided.
	habitIDs := parseHabitIDs(in.RelatedHabitIds)
	if len(habitIDs) > 0 {
		if err := l.svcCtx.Repo.Goals.LinkGoalHabitsBatch(ctx, goal.ID, habitIDs); err != nil {
			l.Errorf("Failed to link habits to goal: %v", err)
			// Non-fatal: goal was created, just without habit links.
		}
	}

	// Create initial milestones for milestone-type goals.
	var milestones []db.GoalMilestone
	if measurement == MeasurementMilestone && len(in.MilestoneTitles) > 0 {
		milestones = make([]db.GoalMilestone, 0, len(in.MilestoneTitles))
		for i, title := range in.MilestoneTitles {
			m, mErr := l.svcCtx.Repo.Goals.CreateGoalMilestone(ctx, goal.ID, title, int32(i))
			if mErr != nil {
				l.Errorf("Failed to create milestone: %v", mErr)
				continue
			}
			milestones = append(milestones, m)
		}
	}

	// Recompute progress for all non-manual types. numeric goals may have
	// current_value already at target (100%), habit goals derive from check-ins,
	// milestone goals from the initial milestones. binary starts at 0 (not
	// completed) which matches the DB default, but recompute is harmless.
	if measurement != MeasurementManual {
		goal, err = recomputeAndPersist(ctx, l.svcCtx, goal.ID)
		if err != nil {
			l.Errorf("Failed to recompute goal progress: %v", err)
		}
		if measurement == MeasurementMilestone && milestones == nil {
			milestones, _ = l.svcCtx.Repo.Goals.ListGoalMilestones(ctx, goal.ID)
		}
	}

	l.svcCtx.InvalidatePersonalizationContext(ctx, userID)

	return &client.CreateGoalResponse{
		Goal: goalToProto(goal, in.RelatedHabitIds, milestones),
	}, nil
}
