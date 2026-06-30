package goalslogic

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
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

	err = l.svcCtx.Repo.Goals.DeleteGoal(ctx, goalID)
	if err != nil {
		l.Errorf("Failed to delete goal: %v", err)
		return nil, status.Error(codes.Internal, "failed to delete goal")
	}

	// Invalidate the cached personalization context for the owning user. The
	// goal row is gone, so derive the user from the authenticated principal.
	if p, ok := principal.PrincipalFrom(ctx); ok {
		if uid, pErr := uuid.Parse(p.UserID); pErr == nil {
			l.svcCtx.InvalidatePersonalizationContext(ctx, uid)
		}
	}

	return &client.DeleteGoalResponse{
		Success: true,
	}, nil
}
