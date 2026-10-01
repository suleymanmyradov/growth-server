package personalizationservicelogic

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type CreatePlanAdjustmentSuggestionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreatePlanAdjustmentSuggestionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreatePlanAdjustmentSuggestionLogic {
	return &CreatePlanAdjustmentSuggestionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreatePlanAdjustmentSuggestionLogic) CreatePlanAdjustmentSuggestion(in *client.CreatePlanAdjustmentSuggestionRequest) (*client.CreatePlanAdjustmentSuggestionResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "CreatePlanAdjustmentSuggestionLogic.CreatePlanAdjustmentSuggestion")
	defer span.End()

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Check plan limit enforcement (auto-create free subscription if missing).
	// EntitlementsOrFreeFallback enforces Free-plan limits when the
	// subscription row can't be loaded; an error means even the fallback
	// failed, so the request is rejected rather than skipping enforcement.
	entitlements, entErr := l.svcCtx.Repo.Billing.EntitlementsOrFreeFallback(ctx, userID)
	if entErr != nil {
		l.Errorf("CreatePlanAdjustmentSuggestion: entitlement check failed closed for user %s: %v", userID, entErr)
		return nil, status.Error(codes.Internal, "failed to verify plan limits")
	}
	if !entitlements.CanCreatePlanAdjustment {
		return nil, status.Error(codes.FailedPrecondition, "PLAN_LIMIT_REACHED:plan_adjustments:plan_adjustments")
	}

	var goalID, habitID uuid.NullUUID
	if in.GoalId != "" {
		goalUUID, err := uuid.Parse(in.GoalId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid goal ID")
		}
		goalID = uuid.NullUUID{UUID: goalUUID, Valid: true}
	}
	if in.HabitId != "" {
		habitUUID, err := uuid.Parse(in.HabitId)
		if err != nil {
			return nil, status.Error(codes.InvalidArgument, "invalid habit ID")
		}
		habitID = uuid.NullUUID{UUID: habitUUID, Valid: true}
	}

	var metadata json.RawMessage
	if in.MetadataJson != "" {
		metadata = json.RawMessage(in.MetadataJson)
		// Validate JSON
		if !json.Valid(metadata) {
			return nil, status.Error(codes.InvalidArgument, "invalid metadata JSON")
		}
	} else {
		metadata = json.RawMessage("{}")
	}

	// Validate goal ownership if goal ID is provided
	if goalID.Valid {
		goal, err := l.svcCtx.Repo.Goals.GetGoalByID(ctx, goalID.UUID)
		if err != nil {
			return nil, status.Error(codes.NotFound, "goal not found")
		}
		if goal.UserID != userID {
			return nil, status.Error(codes.PermissionDenied, "goal does not belong to user")
		}
	}

	// Validate habit ownership if habit ID is provided
	if habitID.Valid {
		timezone := "UTC"
		if prefs, pErr := l.svcCtx.Repo.UserPreferences.GetUserPreferences(ctx, userID); pErr == nil {
			timezone = prefs.Timezone
		}
		habit, err := l.svcCtx.Repo.Habits.GetHabitByID(ctx, habitID.UUID, timezone)
		if err != nil {
			return nil, status.Error(codes.NotFound, "habit not found")
		}
		if habit.UserID != userID {
			return nil, status.Error(codes.PermissionDenied, "habit does not belong to user")
		}
	}

	var suggestion db.PlanAdjustment
	err = l.svcCtx.RunInTx(ctx, userID.String(), func(txRepo *repository.Repository) error {
		var txErr error
		suggestion, txErr = txRepo.PlanAdjustmentSuggestions.CreatePlanAdjustmentSuggestion(ctx, db.CreatePlanAdjustmentSuggestionParams{
			UserID:         userID,
			GoalID:         goalID,
			HabitID:        habitID,
			Source:         (in.Source),
			AdjustmentType: (in.AdjustmentType),
			Reason:         in.Reason,
			Suggestion:     in.Suggestion,
			Metadata:       metadata,
		})
		if txErr != nil {
			return txErr
		}
		habitIDStr := ""
		if habitID.Valid {
			habitIDStr = habitID.UUID.String()
		}
		goalIDStr := ""
		if goalID.Valid {
			goalIDStr = goalID.UUID.String()
		}
		env, envErr := events.NewEnvelope(events.TypePlanAdjustmentCreated, events.PlanAdjustmentCreated{
			UserID:         userID.String(),
			SuggestionID:   suggestion.ID.String(),
			HabitID:        habitIDStr,
			GoalID:         goalIDStr,
			Source:         in.Source,
			AdjustmentType: in.AdjustmentType,
		})
		if envErr != nil {
			return fmt.Errorf("build plan_adjustment_created envelope: %w", envErr)
		}
		return txRepo.EventOutbox.Enqueue(ctx, env)
	})
	if err != nil {
		l.Errorf("failed to create plan adjustment suggestion: %v", err)
		return nil, status.Error(codes.Internal, "failed to create plan adjustment suggestion")
	}

	return &client.CreatePlanAdjustmentSuggestionResponse{
		Suggestion: dbPlanAdjustmentSuggestionToProto(suggestion),
	}, nil
}
