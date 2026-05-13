package goalslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
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
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	offset := (in.Page - 1) * limit
	if offset < 0 {
		offset = 0
	}

	goals, err := l.svcCtx.Repo.Goals.ListGoals(l.ctx, userID, limit, offset)
	if err != nil {
		l.Errorf("Failed to list goals: %v", err)
		return nil, err
	}

	total, err := l.svcCtx.Repo.Goals.CountGoalsByUser(l.ctx, userID)
	if err != nil {
		l.Errorf("Failed to count goals: %v", err)
		return nil, err
	}

	var pbGoals []*client.Goal
	for _, g := range goals {
		pbGoals = append(pbGoals, goalToProto(g))
	}

	return &client.ListGoalsResponse{
		Goals: pbGoals,
		Total: int32(total),
	}, nil
}
