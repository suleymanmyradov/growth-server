package habitslogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreateHabitLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateHabitLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateHabitLogic {
	return &CreateHabitLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateHabitLogic) CreateHabit(in *client.CreateHabitRequest) (*client.CreateHabitResponse, error) {
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
return nil, status.Error(codes.Internal, "invalid user id")
	}

	// Check plan limit enforcement (auto-create free subscription if missing)
	sub, subErr := l.svcCtx.Repo.Billing.GetOrCreateUserSubscription(l.ctx, userID)
	if subErr == nil {
		entitlements, computeErr := l.svcCtx.Repo.Billing.ComputeEntitlements(l.ctx, sub, userID)
		if computeErr == nil && !entitlements.CanCreateHabit {
			st := status.New(codes.FailedPrecondition, "plan limit reached")
			st, _ = st.WithDetails(&client.PlanLimitDetail{
				Limit:          "active_habits",
				UpgradeTrigger: "habit_limit",
			})
			return nil, st.Err()
		}
	}

	name, desc, category, uid := protoToHabitParams(in.Name, in.Description, in.Category, userID)
	habit, err := l.svcCtx.Repo.Habits.CreateHabit(l.ctx, name, desc, category, uid)
	if err != nil {
		l.Errorf("Failed to create habit: %v", err)
return nil, status.Error(codes.Internal, "failed to create habit")
	}

	return &client.CreateHabitResponse{
		Habit: habitToProto(habit),
	}, nil
}
