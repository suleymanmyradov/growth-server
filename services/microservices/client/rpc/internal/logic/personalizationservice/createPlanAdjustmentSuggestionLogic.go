package personalizationservicelogic

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
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
	p, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Check plan limit enforcement (auto-create free subscription if missing)
	sub, subErr := l.svcCtx.Repo.Billing.GetOrCreateUserSubscription(l.ctx, userID)
	if subErr == nil {
		entitlements, computeErr := l.svcCtx.Repo.Billing.ComputeEntitlements(l.ctx, sub, userID)
		if computeErr == nil && !entitlements.CanCreatePlanAdjustment {
			return nil, status.Error(codes.FailedPrecondition, "PLAN_LIMIT_REACHED:plan_adjustments:plan_adjustments")
		}
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
		goal, err := l.svcCtx.Repo.Goals.GetGoalByID(l.ctx, goalID.UUID)
		if err != nil {
			return nil, status.Error(codes.NotFound, "goal not found")
		}
		if goal.UserID != userID {
			return nil, status.Error(codes.PermissionDenied, "goal does not belong to user")
		}
	}

	// Validate habit ownership if habit ID is provided
	if habitID.Valid {
		habit, err := l.svcCtx.Repo.Habits.GetHabitByID(l.ctx, habitID.UUID)
		if err != nil {
			return nil, status.Error(codes.NotFound, "habit not found")
		}
		if habit.UserID != userID {
			return nil, status.Error(codes.PermissionDenied, "habit does not belong to user")
		}
	}

	suggestion, err := l.svcCtx.Repo.PlanAdjustmentSuggestions.CreatePlanAdjustmentSuggestion(l.ctx, db.CreatePlanAdjustmentSuggestionParams{
		UserID:         userID,
		GoalID:         goalID,
		HabitID:        habitID,
		Source:         db.PlanAdjustmentSourceType(in.Source),
		AdjustmentType: db.PlanAdjustmentTypeType(in.AdjustmentType),
		Reason:         in.Reason,
		Suggestion:     in.Suggestion,
		Metadata:       metadata,
	})
	if err != nil {
		l.Errorf("failed to create plan adjustment suggestion: %v", err)
		return nil, status.Error(codes.Internal, "failed to create plan adjustment suggestion")
	}

	return &client.CreatePlanAdjustmentSuggestionResponse{
		Suggestion: dbPlanAdjustmentSuggestionToProto(suggestion),
	}, nil
}
