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
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, err
	}

	params := protoToGoalParams(in.Title, in.Description, in.Category, in.DueDate, userID)
	goal, err := l.svcCtx.Repo.Goals.CreateGoal(l.ctx, params)
	if err != nil {
		l.Errorf("Failed to create goal: %v", err)
		return nil, err
	}

	return &client.CreateGoalResponse{
		Goal: goalToProto(goal),
	}, nil
}
