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

type LogGoalValueLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewLogGoalValueLogic(ctx context.Context, svcCtx *svc.ServiceContext) *LogGoalValueLogic {
	return &LogGoalValueLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *LogGoalValueLogic) LogGoalValue(in *client.LogGoalValueRequest) (*client.LogGoalValueResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "LogGoalValueLogic.LogGoalValue")
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
		l.Errorf("Invalid goal ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid goal id")
	}

	// Verify ownership and measurement type before mutating.
	existing, err := l.svcCtx.Repo.Goals.GetGoalByID(ctx, goalID)
	if err != nil {
		l.Errorf("Failed to get goal: %v", err)
		return nil, status.Error(codes.NotFound, "goal not found")
	}
	if existing.UserID.String() != p.UserID {
		return nil, status.Error(codes.PermissionDenied, "access denied")
	}
	if existing.Measurement != MeasurementNumeric {
		return nil, status.Error(codes.FailedPrecondition, "value logging is only available for numeric goals")
	}

	// Write the new value and recompute progress atomically.
	var goal db.GetGoalRow
	var habitIDs []uuid.UUID
	err = l.svcCtx.RunInTx(ctx, p.UserID, func(txRepo *repository.Repository) error {
		var vErr error
		goal, vErr = txRepo.Goals.LogGoalValue(ctx, goalID, floatToNumeric(in.Value))
		if vErr != nil {
			return status.Error(codes.Internal, "failed to log goal value")
		}
		var rErr error
		goal, rErr = RecomputeGoalProgressWithRepo(ctx, txRepo.Goals, txRepo.CheckIns, goalID)
		if rErr != nil {
			return fmt.Errorf("recompute goal progress: %w", rErr)
		}
		habitIDs, _ = txRepo.Goals.ListGoalHabitIDsByGoal(ctx, goalID)
		return nil
	})
	if err != nil {
		if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
			return nil, st.Err()
		}
		l.Errorf("Failed to log goal value: %v", err)
		return nil, status.Error(codes.Internal, "failed to log goal value")
	}

	l.svcCtx.InvalidatePersonalizationContext(ctx, goal.UserID)

	return &client.LogGoalValueResponse{
		Goal: goalToProto(goal, habitUUIDsToStrings(habitIDs), nil),
	}, nil
}
