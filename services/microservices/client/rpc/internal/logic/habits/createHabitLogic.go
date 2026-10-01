package habitslogic

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	commonlogic "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/logic/common"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
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
	// testTxRunner is only set in tests to inject a no-op transaction runner.
	testTxRunner pgxTxRunner
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
	// EntitlementsOrFreeFallback enforces Free-plan limits when the
	// subscription row can't be loaded; an error means even the fallback
	// failed, so the request is rejected rather than skipping enforcement.
	entitlements, entErr := l.svcCtx.Repo.Billing.EntitlementsOrFreeFallback(ctx, userID)
	if entErr != nil {
		l.Errorf("CreateHabit: entitlement check failed closed for user %s: %v", userID, entErr)
		return nil, status.Error(codes.Internal, "failed to verify plan limits")
	}
	if !entitlements.CanCreateHabit {
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

	name, desc, category, uid := protoToHabitParams(in.Name, in.Description, in.Category, userID)

	// Habit row + habit_created event commit together (outbox inside the tx).
	// Notifications consumes habit_created to update active_habit_count in its
	// reminder_state read model, so losing the event would silently break
	// habit_reminder reminders for this user.
	var habit db.GetHabitRow
	err = runInTx(l.svcCtx, l.testTxRunner, ctx, userID.String(), func(txRepo *repository.Repository) error {
		var txErr error
		habit, txErr = txRepo.Habits.CreateHabit(ctx, name, desc, category, uid)
		if txErr != nil {
			return txErr
		}
		env, envErr := events.NewEnvelope(events.TypeHabitCreated, events.HabitCreated{
			UserID:    userID.String(),
			HabitID:   habit.ID.String(),
			HabitName: habit.Name,
		})
		if envErr != nil {
			return fmt.Errorf("envelope: %w", envErr)
		}
		return txRepo.EventOutbox.Enqueue(ctx, env)
	})
	if err != nil {
		l.Errorf("Failed to create habit: %v", err)
		return nil, status.Error(codes.Internal, "failed to create habit")
	}

	l.svcCtx.InvalidatePersonalizationContext(ctx, userID)

	return &client.CreateHabitResponse{
		Habit: habitToProto(habit, 0, nil), // new habit has no check-ins → streak 0
	}, nil
}
