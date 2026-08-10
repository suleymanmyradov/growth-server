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

type ToggleMilestoneLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewToggleMilestoneLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ToggleMilestoneLogic {
	return &ToggleMilestoneLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ToggleMilestoneLogic) ToggleMilestone(in *client.ToggleMilestoneRequest) (*client.ToggleMilestoneResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ToggleMilestoneLogic.ToggleMilestone")
	defer span.End()
	if in == nil || in.GoalId == "" || in.MilestoneId == "" {
		return nil, status.Error(codes.InvalidArgument, "goal ID and milestone ID are required")
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
	milestoneID, err := uuid.Parse(in.MilestoneId)
	if err != nil {
		l.Errorf("Invalid milestone ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid milestone id")
	}

	// Verify goal ownership before toggling the milestone. The milestone row
	// has no user_id — ownership is established via the goal.
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

	// Toggle the milestone and recompute progress atomically inside a
	// transaction so a recompute failure rolls back the toggle.
	var goal db.GetGoalRow
	var habitIDs []uuid.UUID
	var milestones []db.GoalMilestone
	err = l.svcCtx.RunInTx(ctx, p.UserID, func(txRepo *repository.Repository) error {
		if _, tErr := txRepo.Goals.ToggleGoalMilestone(ctx, milestoneID, goalID); tErr != nil {
			return status.Error(codes.Internal, "failed to toggle milestone")
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
		l.Errorf("Failed to toggle milestone: %v", err)
		return nil, status.Error(codes.Internal, "failed to toggle milestone")
	}

	l.svcCtx.InvalidatePersonalizationContext(ctx, goal.UserID)

	return &client.ToggleMilestoneResponse{
		Goal: goalToProto(goal, habitUUIDsToStrings(habitIDs), milestones),
	}, nil
}
