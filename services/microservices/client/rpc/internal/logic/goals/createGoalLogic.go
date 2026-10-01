package goalslogic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	commonlogic "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/logic/common"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
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
	// EntitlementsOrFreeFallback enforces Free-plan limits when the
	// subscription row can't be loaded; an error means even the fallback
	// failed, so the request is rejected rather than skipping enforcement.
	entitlements, entErr := l.svcCtx.Repo.Billing.EntitlementsOrFreeFallback(ctx, userID)
	if entErr != nil {
		l.Errorf("CreateGoal: entitlement check failed closed for user %s: %v", userID, entErr)
		return nil, status.Error(codes.Internal, "failed to verify plan limits")
	}
	if !entitlements.CanCreateGoal {
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

	habitIDs := parseHabitIDs(in.RelatedHabitIds)
	if len(habitIDs) > 0 {
		if err := validateHabitOwnership(ctx, l.svcCtx.Repo.Habits, userID, habitIDs); err != nil {
			return nil, err
		}
	}

	params := protoToGoalParams(in.Title, in.Description, in.Category, in.DueDate, userID,
		measurement, in.StartValue, in.CurrentValue, in.TargetValue, in.Unit)

	// Create the goal, habit links, milestones, and initial progress recompute
	// atomically — a habit-goal returned 200 with zero links and 0% progress
	// forever when these steps failed silently outside a transaction.
	var goal db.GetGoalRow
	var milestones []db.GoalMilestone
	err = l.svcCtx.RunInTx(ctx, p.UserID, func(txRepo *repository.Repository) error {
		var cErr error
		goal, cErr = txRepo.Goals.CreateGoal(ctx, params)
		if cErr != nil {
			return fmt.Errorf("create goal: %w", cErr)
		}

		// Link habits to the new goal if any were provided.
		if len(habitIDs) > 0 {
			if lErr := txRepo.Goals.LinkGoalHabitsBatch(ctx, goal.ID, habitIDs); lErr != nil {
				return fmt.Errorf("link habits to goal: %w", lErr)
			}
		}

		// Create initial milestones for milestone-type goals.
		if measurement == MeasurementMilestone && len(in.MilestoneTitles) > 0 {
			milestones = make([]db.GoalMilestone, 0, len(in.MilestoneTitles))
			for i, title := range in.MilestoneTitles {
				m, mErr := txRepo.Goals.CreateGoalMilestone(ctx, goal.ID, title, int32(i))
				if mErr != nil {
					return fmt.Errorf("create goal milestone: %w", mErr)
				}
				milestones = append(milestones, m)
			}
		}

		// Recompute progress for all non-manual types. numeric goals may have
		// current_value already at target (100%), habit goals derive from check-ins,
		// milestone goals from the initial milestones. binary starts at 0 (not
		// completed) which matches the DB default, but recompute is harmless.
		if measurement != MeasurementManual {
			var rErr error
			goal, rErr = RecomputeGoalProgressWithRepo(ctx, txRepo.Goals, txRepo.CheckIns, goal.ID)
			if rErr != nil {
				return fmt.Errorf("recompute goal progress: %w", rErr)
			}
			if measurement == MeasurementMilestone && milestones == nil {
				var lErr error
				milestones, lErr = txRepo.Goals.ListGoalMilestones(ctx, goal.ID)
				if lErr != nil {
					return fmt.Errorf("list goal milestones: %w", lErr)
				}
			}
		}

		// goal_created event goes into the outbox in this transaction.
		// DeadlineAt is included so the notifications consumer can schedule a
		// goal_deadline reminder when the goal has a due date.
		deadlineAt := ""
		if goal.DueDate.Valid {
			deadlineAt = goal.DueDate.Time.Format(time.RFC3339)
		}
		env, envErr := events.NewEnvelope(events.TypeGoalCreated, events.GoalCreated{
			UserID:     userID.String(),
			GoalID:     goal.ID.String(),
			Title:      goal.Title,
			Category:   goal.Category,
			DeadlineAt: deadlineAt,
		})
		if envErr != nil {
			return fmt.Errorf("build goal_created envelope: %w", envErr)
		}
		if err := txRepo.EventOutbox.Enqueue(ctx, env); err != nil {
			return fmt.Errorf("enqueue goal_created event: %w", err)
		}
		return nil
	})
	if err != nil {
		l.Errorf("Failed to create goal: %v", err)
		return nil, status.Error(codes.Internal, "failed to create goal")
	}

	l.svcCtx.InvalidatePersonalizationContext(ctx, userID)

	// Log goal_created activity for metrics/activation tracking.
	activityTitle := goal.Title
	activityDesc := "Created goal: " + goal.Title
	if _, aErr := l.svcCtx.Repo.Activities.CreateActivity(ctx, db.CreateActivityParams{
		Type:        "goal_created",
		Title:       activityTitle,
		Description: &activityDesc,
		Metadata:    json.RawMessage("{}"),
		UserID:      userID,
	}); aErr != nil {
		l.Errorf("Failed to log goal_created activity: %v", aErr)
	}

	return &client.CreateGoalResponse{
		Goal: goalToProto(goal, in.RelatedHabitIds, milestones),
	}, nil
}
