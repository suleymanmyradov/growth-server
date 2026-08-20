package goalslogic

import (
	"context"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
)

type DeleteGoalLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewDeleteGoalLogic(ctx context.Context, svcCtx *svc.ServiceContext) *DeleteGoalLogic {
	return &DeleteGoalLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *DeleteGoalLogic) DeleteGoal(in *client.DeleteGoalRequest) (*client.DeleteGoalResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "DeleteGoalLogic.DeleteGoal")
	defer span.End()
	goalID, err := uuid.Parse(in.GoalId)
	if err != nil {
		l.Errorf("Invalid goal ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid goal id")
	}

	// Ownership check: verify the caller owns this goal before deleting.
	// DeleteGoal SQL has no WHERE user_id clause, so without this check any
	// authenticated user could delete any other user's goal by ID.
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

	err = l.svcCtx.Repo.Goals.DeleteGoal(ctx, goalID)
	if err != nil {
		l.Errorf("Failed to delete goal: %v", err)
		return nil, status.Error(codes.Internal, "failed to delete goal")
	}

	// Invalidate the cached personalization context for the owning user.
	if uid, pErr := uuid.Parse(p.UserID); pErr == nil {
		l.svcCtx.InvalidatePersonalizationContext(ctx, uid)
	}

	// Fire-and-forget publish goal_deleted event for analytics/metrics.
	if l.svcCtx.EventsPub != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			env, err := events.NewEnvelope(events.TypeGoalDeleted, events.GoalDeleted{
				UserID: p.UserID,
				GoalID: goalID.String(),
			})
			if err != nil {
				logx.Errorf("envelope: %v", err)
				return
			}
			if err := l.svcCtx.EventsPub.Publish(ctx, env); err != nil {
				logx.Errorf("publish goal_deleted event: %v", err)
			}
		}()
	}

	return &client.DeleteGoalResponse{
		Success: true,
	}, nil
}
