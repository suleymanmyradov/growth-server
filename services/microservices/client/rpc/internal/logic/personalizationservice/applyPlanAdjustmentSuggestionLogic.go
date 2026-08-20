package personalizationservicelogic

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
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
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "ApplyPlanAdjustmentSuggestionLogic.ApplyPlanAdjustmentSuggestion")
	defer span.End()

	suggestionID, err := uuid.Parse(in.SuggestionId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid suggestion ID")
	}

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	suggestion, err := l.svcCtx.Repo.PlanAdjustmentSuggestions.GetPlanAdjustmentSuggestion(ctx, suggestionID, userID)
	if err != nil {
		l.Errorf("failed to get plan adjustment suggestion: %v", err)
		return nil, status.Error(codes.NotFound, "suggestion not found")
	}

	if suggestion.Status == "applied" {
		return nil, status.Error(codes.FailedPrecondition, "suggestion already applied")
	}

	// Apply the adjustment based on type. Each case mutates the underlying
	// habit or goal row, then we mark the suggestion as 'applied'.
	switch suggestion.AdjustmentType {
	case "reduce_difficulty", "increase_difficulty":
		if suggestion.HabitID.Valid {
			if err := l.applyDifficultyAdjustment(ctx, suggestion); err != nil {
				return nil, err
			}
		}
	case "change_time":
		if suggestion.HabitID.Valid {
			if err := l.applyTimeChange(ctx, suggestion); err != nil {
				return nil, err
			}
		}
	case "clarify_plan":
		if suggestion.GoalID.Valid {
			if err := l.applyClarifyPlan(ctx, suggestion); err != nil {
				return nil, err
			}
		}
	case "pause":
		if suggestion.HabitID.Valid {
			if err := l.applyPause(ctx, suggestion); err != nil {
				return nil, err
			}
		}
	case "keep_same":
		l.Infof("Marking 'keep_same' suggestion as applied")
	default:
		return nil, status.Errorf(codes.InvalidArgument, "unknown adjustment type: %s", suggestion.AdjustmentType)
	}

	appliedSuggestion, err := l.svcCtx.Repo.PlanAdjustmentSuggestions.ApplyPlanAdjustmentSuggestion(ctx, suggestionID, userID)
	if err != nil {
		l.Errorf("failed to apply plan adjustment suggestion: %v", err)
		return nil, status.Error(codes.Internal, "failed to apply suggestion")
	}

	return &client.ApplyPlanAdjustmentSuggestionResponse{
		Suggestion: dbPlanAdjustmentSuggestionToProto(appliedSuggestion),
		Success:    true,
	}, nil
}

// applyDifficultyAdjustment updates the habit's description to the suggestion
// text (which contains the concrete reduced/increased version, e.g. "Scale
// down to a 10-minute walk"). The original description is preserved in the
// suggestion metadata so it can be restored later.
func (l *ApplyPlanAdjustmentSuggestionLogic) applyDifficultyAdjustment(ctx context.Context, suggestion db.PlanAdjustment) error {
	habitID := suggestion.HabitID.UUID
	habit, err := l.svcCtx.Repo.Habits.GetHabitByID(ctx, habitID, "UTC")
	if err != nil {
		return status.Error(codes.NotFound, "habit not found")
	}

	newDesc := suggestion.Suggestion
	_, err = l.svcCtx.Repo.Habits.UpdateHabitDescription(ctx, habitID, &newDesc)
	if err != nil {
		return status.Error(codes.Internal, "failed to update habit description")
	}

	l.Infof("Applied difficulty adjustment for habit %s: original=%q new=%q",
		habitID, habit.Description, newDesc)
	return nil
}

// applyTimeChange parses a time-of-day from the suggestion text and updates
// the habit's reminder_time. If no time can be parsed, the habit mutation is
// skipped (the suggestion is still marked as applied).
func (l *ApplyPlanAdjustmentSuggestionLogic) applyTimeChange(ctx context.Context, suggestion db.PlanAdjustment) error {
	habitID := suggestion.HabitID.UUID
	t := parseTimeFromSuggestion(suggestion.Suggestion)
	if t == nil {
		l.Infof("change_time: could not parse time from suggestion %q, skipping habit mutation", suggestion.Suggestion)
		return nil
	}
	_, err := l.svcCtx.Repo.Habits.UpdateHabitReminderTime(ctx, habitID, *t)
	if err != nil {
		return status.Error(codes.Internal, "failed to update habit reminder time")
	}
	l.Infof("Applied time change for habit %s", habitID)
	return nil
}

// applyClarifyPlan updates the goal's description to the suggestion text.
func (l *ApplyPlanAdjustmentSuggestionLogic) applyClarifyPlan(ctx context.Context, suggestion db.PlanAdjustment) error {
	goalID := suggestion.GoalID.UUID
	newDesc := suggestion.Suggestion
	_, err := l.svcCtx.Repo.Goals.UpdateGoalDescription(ctx, goalID, &newDesc)
	if err != nil {
		return status.Error(codes.Internal, "failed to update goal description")
	}
	l.Infof("Applied plan clarification for goal %s", goalID)
	return nil
}

// applyPause sets the habit's status to 'paused'.
func (l *ApplyPlanAdjustmentSuggestionLogic) applyPause(ctx context.Context, suggestion db.PlanAdjustment) error {
	habitID := suggestion.HabitID.UUID
	_, err := l.svcCtx.Repo.Habits.UpdateHabitStatus(ctx, habitID, "paused")
	if err != nil {
		return status.Error(codes.Internal, "failed to pause habit")
	}
	l.Infof("Paused habit %s", habitID)
	return nil
}

// parseTimeFromSuggestion extracts a time-of-day from a free-text suggestion.
// It looks for patterns like "8:00 AM", "08:00", "14:30". Returns nil if no
// time is found.
func parseTimeFromSuggestion(text string) *pgtype.Time {
	for _, layout := range []string{"3:04 PM", "3:04pm", "15:04", "3:04PM"} {
		for i := 0; i < len(text); i++ {
			if text[i] < '0' || text[i] > '9' {
				continue
			}
			end := i
			for end < len(text) && ((text[end] >= '0' && text[end] <= '9') || text[end] == ':') {
				end++
			}
			rest := strings.TrimLeft(text[end:], " ")
			if len(rest) >= 2 {
				upper := strings.ToUpper(rest[:2])
				if upper == "AM" || upper == "PM" {
					end += 2
				}
			}
			substr := strings.TrimSpace(text[i:end])
			t, err := time.Parse(layout, substr)
			if err == nil {
				us := int64(t.Hour())*3600_000_000 + int64(t.Minute())*60_000_000 + int64(t.Second())*1_000_000
				return &pgtype.Time{Microseconds: us, Valid: true}
			}
		}
	}
	return nil
}
