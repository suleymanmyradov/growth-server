package weeklyreviewservicelogic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SaveWeeklyReviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewSaveWeeklyReviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *SaveWeeklyReviewLogic {
	return &SaveWeeklyReviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

// SaveWeeklyReview persists the AI-generated weekly review to the database,
// logs activity, and creates plan adjustment suggestions in the background.
func (l *SaveWeeklyReviewLogic) SaveWeeklyReview(in *client.SaveWeeklyReviewRequest) (*client.SaveWeeklyReviewResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "SaveWeeklyReviewLogic.SaveWeeklyReview")
	defer span.End()

	if in.Data == nil {
		return nil, status.Error(codes.InvalidArgument, "data is required")
	}

	data := in.Data
	userID, err := uuid.Parse(data.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	weekStart, err := time.ParseInLocation("2006-01-02", data.WeekStart, time.UTC)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid weekStart")
	}

	// Serialize the DB-specific JSON blobs from the prepared data.
	moodSummaryJSON, err := json.Marshal(data.MoodSummary)
	if err != nil {
		l.Errorf("failed to marshal mood summary: %v", err)
		return nil, status.Error(codes.Internal, "failed to serialize mood summary")
	}
	energySummaryJSON, err := json.Marshal(data.EnergySummary)
	if err != nil {
		l.Errorf("failed to marshal energy summary: %v", err)
		return nil, status.Error(codes.Internal, "failed to serialize energy summary")
	}

	// habitBreakdownsForDB: convert the proto habit breakdowns to the DB format.
	habitBreakdownsForDB := make([]habitBreakdownDB, len(data.HabitBreakdowns))
	for i, h := range data.HabitBreakdowns {
		lastCheckInAt := ""
		if h.LastCheckInAt > 0 {
			lastCheckInAt = time.Unix(h.LastCheckInAt, 0).Format(time.RFC3339)
		}
		habitBreakdownsForDB[i] = habitBreakdownDB{
			HabitID:        h.HabitId,
			HabitName:      h.HabitName,
			Category:       h.Category,
			TotalCheckIns:  int(h.TotalCheckIns),
			CompletedCount: int(h.CompletedCount),
			MissedCount:    int(h.MissedCount),
			CompletionRate: h.CompletionRate,
			LastCheckInAt:  lastCheckInAt,
		}
	}
	habitBreakdownJSON, err := json.Marshal(habitBreakdownsForDB)
	if err != nil {
		l.Errorf("failed to marshal habit breakdown: %v", err)
		return nil, status.Error(codes.Internal, "failed to serialize habit breakdown")
	}

	// Normalize adjustment types before persisting.
	for _, adj := range in.SuggestedAdjustments {
		if adj != nil {
			adj.AdjustmentType = normalizeAdjustmentType(adj.AdjustmentType)
		}
	}

	// Convert to aicoachservice types for DB serialization (backward
	// compatibility: existing DB rows use snake_case JSON keys from the
	// aicoachservice proto types).
	aiAdjustments := make([]*aicoachservice.WeeklyReviewAdjustment, 0, len(in.SuggestedAdjustments))
	for _, adj := range in.SuggestedAdjustments {
		if adj == nil {
			continue
		}
		aiAdjustments = append(aiAdjustments, &aicoachservice.WeeklyReviewAdjustment{
			HabitId:        adj.HabitId,
			HabitName:      adj.HabitName,
			Reason:         adj.Reason,
			Suggestion:     adj.Suggestion,
			AdjustmentType: adj.AdjustmentType,
		})
	}
	suggestedAdjustmentsJSON, err := json.Marshal(aiAdjustments)
	if err != nil {
		l.Errorf("failed to marshal suggested adjustments: %v", err)
		return nil, status.Error(codes.Internal, "failed to serialize suggested adjustments")
	}

	var aiNextWeekPlan *aicoachservice.NextWeekPlan
	if in.NextWeekPlan != nil {
		aiNextWeekPlan = &aicoachservice.NextWeekPlan{
			Focus:           in.NextWeekPlan.Focus,
			Commitments:     in.NextWeekPlan.Commitments,
			Risks:           in.NextWeekPlan.Risks,
			RecoveryActions: in.NextWeekPlan.RecoveryActions,
		}
	}
	nextWeekPlanJSON, err := json.Marshal(aiNextWeekPlan)
	if err != nil {
		l.Errorf("failed to marshal next week plan: %v", err)
		return nil, status.Error(codes.Internal, "failed to serialize next week plan")
	}

	weekStartDate := pgtype.Date{Time: weekStart, Valid: true}

	var completionRate pgtype.Numeric
	if err := completionRate.Scan(fmt.Sprintf("%.2f", data.CompletionRate)); err != nil {
		l.Infof("failed to scan completion rate: %v", err)
	}

	var bestDay *string
	if data.BestDay != "" {
		bestDay = &data.BestDay
	}
	var hardestDay *string
	if data.HardestDay != "" {
		hardestDay = &data.HardestDay
	}
	var topBlocker *string
	if data.TopBlocker != "" {
		topBlocker = &data.TopBlocker
	}
	var aiSummaryPtr *string
	if in.AiSummary != "" {
		aiSummaryPtr = &in.AiSummary
	}

	params := db.CreateWeeklyReviewParams{
		UserID:               userID,
		WeekStart:            weekStartDate,
		TotalHabits:          data.TotalHabits,
		CompletedCheckIns:    data.CompletedCheckIns,
		MissedCheckIns:       data.MissedCheckIns,
		CompletionRate:       completionRate,
		BestDay:              bestDay,
		HardestDay:           hardestDay,
		TopBlocker:           topBlocker,
		MoodSummary:          moodSummaryJSON,
		EnergySummary:        energySummaryJSON,
		HabitBreakdown:       habitBreakdownJSON,
		AiSummary:            aiSummaryPtr,
		SuggestedAdjustments: suggestedAdjustmentsJSON,
		NextWeekPlan:         nextWeekPlanJSON,
	}

	review, err := l.svcCtx.Repo.WeeklyReviews.CreateWeeklyReview(ctx, params)
	if err != nil {
		l.Errorf("failed to save weekly review: %v", err)
		return nil, status.Error(codes.Internal, "failed to save weekly review")
	}

	// Log activity
	weekLabel := weekStart.Format("Jan 2, 2006")
	desc := fmt.Sprintf("Generated weekly review for %s", weekLabel)
	meta, _ := json.Marshal(map[string]string{
		"reviewId":  review.ID.String(),
		"weekStart": weekStart.Format("2006-01-02"),
	})
	if _, err := l.svcCtx.Repo.Activities.CreateActivity(ctx, db.CreateActivityParams{
		Type:        "weekly_review_generated",
		Title:       fmt.Sprintf("Weekly review for %s", weekLabel),
		Description: &desc,
		Metadata:    meta,
		UserID:      userID,
	}); err != nil {
		l.Errorf("Failed to log weekly_review_generated activity: %v", err)
	}

	// Create plan adjustment suggestions in background
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logx.Errorf("panic while creating plan adjustment suggestions: %v", r)
			}
		}()

		bgCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		for _, adjustment := range in.SuggestedAdjustments {
			if adjustment == nil || adjustment.AdjustmentType == "keep_same" {
				continue
			}

			var goalID, habitID uuid.NullUUID
			if adjustment.HabitId != "" {
				if habitUUID, err := uuid.Parse(adjustment.HabitId); err == nil {
					habitID = uuid.NullUUID{UUID: habitUUID, Valid: true}
				}
			}

			metadata := map[string]string{
				"source":          "weekly_review",
				"week_start":      weekStart.Format(time.RFC3339),
				"habit_name":      adjustment.HabitName,
				"adjustment_type": adjustment.AdjustmentType,
				"ai_generated":    "true",
			}
			metadataJSON, _ := json.Marshal(metadata)

			_, err := l.svcCtx.Repo.PlanAdjustmentSuggestions.CreatePlanAdjustmentSuggestion(bgCtx, db.CreatePlanAdjustmentSuggestionParams{
				UserID:         userID,
				GoalID:         goalID,
				HabitID:        habitID,
				Source:         "weekly_review",
				AdjustmentType: adjustment.AdjustmentType,
				Reason:         adjustment.Reason,
				Suggestion:     adjustment.Suggestion,
				Metadata:       metadataJSON,
				WeekStart:      pgtype.Date{Time: weekStart, Valid: true},
			})
			if err != nil {
				logx.Errorf("failed to create plan adjustment suggestion: %v", err)
			}
		}
	}()

	return &client.SaveWeeklyReviewResponse{Review: dbReviewToProto(review)}, nil
}
