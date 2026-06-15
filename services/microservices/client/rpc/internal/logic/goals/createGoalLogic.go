package goalslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
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

	// Check plan limit enforcement (auto-create free subscription if missing)
	sub, subErr := l.svcCtx.Repo.Billing.GetOrCreateUserSubscription(ctx, userID)
	if subErr == nil {
		entitlements, computeErr := l.svcCtx.Repo.Billing.ComputeEntitlements(ctx, sub, userID)
		if computeErr == nil && !entitlements.CanCreateGoal {
			st := status.New(codes.FailedPrecondition, "plan limit reached")
			st, _ = st.WithDetails(&client.PlanLimitDetail{
				Limit:          "active_goals",
				UpgradeTrigger: "goal_limit",
			})
			return nil, st.Err()
		}
	}

	params := protoToGoalParams(in.Title, in.Description, in.Category, in.DueDate, userID)
	goal, err := l.svcCtx.Repo.Goals.CreateGoal(ctx, params)
	if err != nil {
		l.Errorf("Failed to create goal: %v", err)
return nil, status.Error(codes.Internal, "failed to create goal")
	}

	return &client.CreateGoalResponse{
		Goal: goalToProto(goal),
	}, nil
}
