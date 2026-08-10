package goalslogic

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateMilestoneLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateMilestoneLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateMilestoneLogic {
	return &CreateMilestoneLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateMilestoneLogic) CreateMilestone(in *client.CreateMilestoneRequest) (*client.CreateMilestoneResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "CreateMilestoneLogic.CreateMilestone")
	defer span.End()
	if in == nil || in.GoalId == "" {
		return nil, status.Error(codes.InvalidArgument, "goal ID is required")
	}
	if in.Title == "" {
		return nil, status.Error(codes.InvalidArgument, "milestone title is required")
	}

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}

	goalID, err := uuid.Parse(in.GoalId)
	if err != nil {
		l.Errorf("Invalid goal ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid goal id")
	}

	// Verify ownership and measurement type before mutating. Milestones have
	// no user_id column — ownership comes via the goal join.
	existing, err := l.svcCtx.Repo.Goals.GetGoalByID(ctx, goalID)
	if err != nil {
		l.Errorf("Failed to get goal: %v", err)
		return nil, status.Error(codes.NotFound, "goal not found")
	}
	if existing.UserID.String() != p.UserID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}
	if existing.Measurement != MeasurementMilestone {
		return nil, status.Error(codes.FailedPrecondition, "milestones are only available for milestone goals")
	}

	// Determine the next sort order if not provided (proto int32 default is 0,
	// so we can't distinguish "not sent" from "explicitly 0" — treat 0 as
	// "auto" since sort_order 0 is the natural first position anyway).
	sortOrder := in.SortOrder
	if sortOrder == 0 {
		counts, cErr := l.svcCtx.Repo.Goals.CountGoalMilestones(ctx, goalID)
		if cErr == nil {
			sortOrder = int32(counts.Total)
		}
	}

	// Create the milestone and recompute progress atomically.
	var goal db.GetGoalRow
	var habitIDs []uuid.UUID
	var milestones []db.GoalMilestone
	err = l.svcCtx.RunInTx(ctx, p.UserID, func(txRepo *repository.Repository) error {
		if _, cErr := txRepo.Goals.CreateGoalMilestone(ctx, goalID, in.Title, sortOrder); cErr != nil {
			return status.Error(codes.Internal, "failed to create milestone")
		}
		var rErr error
		goal, rErr = RecomputeGoalProgressWithRepo(ctx, txRepo.Goals, txRepo.CheckIns, goalID)
		if rErr != nil {
			return fmt.Errorf("recompute goal progress: %w", rErr)
		}
		habitIDs, _ = txRepo.Goals.ListGoalHabitIDsByGoal(ctx, goalID)
		milestones, _ = txRepo.Goals.ListGoalMilestones(ctx, goalID)
		return nil
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
			return nil, st.Err()
		}
		l.Errorf("Failed to create milestone: %v", err)
		return nil, status.Error(codes.Internal, "failed to create milestone")
	}

	l.svcCtx.InvalidatePersonalizationContext(ctx, goal.UserID)

	return &client.CreateMilestoneResponse{
		Goal: goalToProto(goal, habitUUIDsToStrings(habitIDs), milestones),
	}, nil
}
