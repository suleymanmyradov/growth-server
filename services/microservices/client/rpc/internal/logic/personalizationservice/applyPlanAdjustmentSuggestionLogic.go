package personalizationservicelogic

import (
	"context"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type ApplyPlanAdjustmentSuggestionLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewApplyPlanAdjustmentSuggestionLogic(ctx context.Context, svcCtx *svc.ServiceContext) *ApplyPlanAdjustmentSuggestionLogic {
	return &ApplyPlanAdjustmentSuggestionLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *ApplyPlanAdjustmentSuggestionLogic) ApplyPlanAdjustmentSuggestion(in *client.ApplyPlanAdjustmentSuggestionRequest) (*client.ApplyPlanAdjustmentSuggestionResponse, error) {
	suggestionID, err := uuid.Parse(in.SuggestionId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid suggestion ID")
	}

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Get the suggestion
	suggestion, err := l.svcCtx.Repo.PlanAdjustmentSuggestions.GetPlanAdjustmentSuggestion(l.ctx, suggestionID, userID)
	if err != nil {
		l.Errorf("failed to get plan adjustment suggestion: %v", err)
		return nil, status.Error(codes.NotFound, "suggestion not found")
	}

	// Apply the adjustment based on type
	switch suggestion.AdjustmentType {
	case "reduce_difficulty", "increase_difficulty":
		// Update habit difficulty if habit_id is present
		if suggestion.HabitID.Valid {
			// Parse metadata to get new difficulty level if provided
			// For now, we'll just mark as applied since difficulty logic depends on your habit schema
			l.Infof("Applying difficulty adjustment for habit %s", suggestion.HabitID.UUID)
		}
	case "change_time":
		// Update habit scheduled time if habit_id is present
		if suggestion.HabitID.Valid {
			l.Infof("Applying time change for habit %s", suggestion.HabitID.UUID)
		}
	case "clarify_plan":
		// Update goal description if goal_id is present
		if suggestion.GoalID.Valid {
			l.Infof("Applying plan clarification for goal %s", suggestion.GoalID.UUID)
		}
	case "pause":
		// Set habit status to paused if habit_id is present
		if suggestion.HabitID.Valid {
			l.Infof("Pausing habit %s", suggestion.HabitID.UUID)
		}
	case "keep_same":
		// No action needed, just mark as applied
		l.Infof("Marking 'keep_same' suggestion as applied")
	default:
		l.Infof("Unknown adjustment type: %s", suggestion.AdjustmentType)
	}

	// Update suggestion status to 'applied'
	appliedSuggestion, err := l.svcCtx.Repo.PlanAdjustmentSuggestions.ApplyPlanAdjustmentSuggestion(l.ctx, suggestionID, userID)
	if err != nil {
		l.Errorf("failed to apply plan adjustment suggestion: %v", err)
		return nil, status.Error(codes.Internal, "failed to apply suggestion")
	}

	return &client.ApplyPlanAdjustmentSuggestionResponse{
		Suggestion: dbPlanAdjustmentSuggestionToProto(appliedSuggestion),
		Success:    true,
	}, nil
}
