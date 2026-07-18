package habitslogic

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	commonlogic "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/logic/common"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "CreateHabitLogic.CreateHabit")
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
	// able to create their initial habits, even if a previous partial onboarding
	// attempt left leftover habits that would otherwise trip the Free plan limit.
	sub, subErr := l.svcCtx.Repo.Billing.GetOrCreateUserSubscription(ctx, userID)
	if subErr == nil {
		entitlements, computeErr := l.svcCtx.Repo.Billing.ComputeEntitlements(ctx, sub, userID)
		if computeErr == nil && !entitlements.CanCreateHabit {
			if !commonlogic.IsOnboardingComplete(ctx, l.svcCtx, userID) {
				l.Infof("CreateHabit: bypassing plan limit for user %s (onboarding not complete)", userID)
			} else {
				st := status.New(codes.FailedPrecondition, "plan limit reached")
				st, _ = st.WithDetails(&client.PlanLimitDetail{
					Limit:          "active_habits",
					UpgradeTrigger: "habit_limit",
				})
				return nil, st.Err()
			}
		}
	}

	name, desc, category, uid := protoToHabitParams(in.Name, in.Description, in.Category, userID)
	habit, err := l.svcCtx.Repo.Habits.CreateHabit(ctx, name, desc, category, uid)
	if err != nil {
		l.Errorf("Failed to create habit: %v", err)
		return nil, status.Error(codes.Internal, "failed to create habit")
	}

	l.svcCtx.InvalidatePersonalizationContext(ctx, userID)

	// Fire-and-forget publish habit_created event so notifications can update
	// its local reminder_state read model (active_habit_count).
	if l.svcCtx.EventsPub != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			env, err := events.NewEnvelope(events.TypeHabitCreated, events.HabitCreated{
				UserID:  userID.String(),
				HabitID: habit.ID.String(),
			})
			if err != nil {
				logx.Errorf("envelope: %v", err)
				return
			}
			if err := l.svcCtx.EventsPub.Publish(ctx, env); err != nil {
				logx.Errorf("publish habit_created event: %v", err)
			}
		}()
	}

	return &client.CreateHabitResponse{
		Habit: habitToProto(habit, 0, nil), // new habit has no check-ins → streak 0
	}, nil
}
