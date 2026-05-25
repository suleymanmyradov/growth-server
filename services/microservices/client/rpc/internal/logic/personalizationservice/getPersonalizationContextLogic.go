package personalizationservicelogic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetPersonalizationContextLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPersonalizationContextLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPersonalizationContextLogic {
	return &GetPersonalizationContextLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetPersonalizationContextLogic) GetPersonalizationContext(in *client.GetPersonalizationContextRequest) (*client.GetPersonalizationContextResponse, error) {
	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Get or create coaching profile
	profile, err := l.svcCtx.Repo.CoachingProfiles.GetCoachingProfile(l.ctx, userID)
	if err != nil {
		// Create default profile if it doesn't exist
		if errors.Is(err, sql.ErrNoRows) {
			profile, err = l.svcCtx.Repo.CoachingProfiles.UpsertCoachingProfile(l.ctx, db.UpsertCoachingProfileParams{
				UserID:               userID,
				AccountabilityStyle:  "balanced",
				PreferredTone:        "supportive",
				DifficultyPreference: "adaptive",
				CommonBlockers:       []byte("[]"),
				CoachingNotes:        []byte("{}"),
			})
			if err != nil {
				l.Errorf("failed to create default coaching profile: %v", err)
				return nil, status.Error(codes.Internal, "failed to create coaching profile")
			}
		} else {
			l.Errorf("failed to get coaching profile: %v", err)
			return nil, status.Error(codes.Internal, "failed to get coaching profile")
		}
	}

	// Get active goals
	goals, err := l.svcCtx.Repo.Goals.ListGoals(l.ctx, userID, 50, 0)
	if err != nil {
		l.Infof("failed to get goals: %v", err)
		goals = []db.Goal{}
	}

	// Get active habits
	habits, err := l.svcCtx.Repo.Habits.ListHabits(l.ctx, userID, 50, 0)
	if err != nil {
		l.Infof("failed to get habits: %v", err)
		habits = []db.Habit{}
	}

	// Get recent check-ins (last 30 days)
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	recentCheckIns, err := l.svcCtx.Repo.CheckIns.GetCheckInHistory(l.ctx, userID, thirtyDaysAgo, time.Now(), 100, 0)
	if err != nil {
		l.Infof("failed to get recent check-ins: %v", err)
		recentCheckIns = []db.CheckIn{}
	}

	// Get latest weekly review
	latestWeeklyReview, err := l.svcCtx.Repo.WeeklyReviews.ListWeeklyReviews(l.ctx, userID, 1, 0)
	if err != nil || len(latestWeeklyReview) == 0 {
		l.Infof("failed to get latest weekly review: %v", err)
		latestWeeklyReview = []db.WeeklyReview{}
	}

	// Get pending plan adjustment suggestions
	pendingSuggestions, err := l.svcCtx.Repo.PlanAdjustmentSuggestions.ListPendingPlanAdjustmentSuggestions(l.ctx, db.ListPendingPlanAdjustmentSuggestionsParams{
		UserID: userID,
		Limit:  20,
		Offset: 0,
	})
	if err != nil {
		l.Infof("failed to get pending suggestions: %v", err)
		pendingSuggestions = []db.PlanAdjustmentSuggestion{}
	}

	// Get user timezone for pattern detection
	userLoc := time.UTC
	settings, err := l.svcCtx.Repo.UserSettings.GetUserSettings(l.ctx, userID)
	if err == nil && settings.Timezone != "" {
		loc, err := time.LoadLocation(settings.Timezone)
		if err != nil {
			l.Infof("invalid timezone %s, using UTC: %v", settings.Timezone, err)
		} else {
			userLoc = loc
		}
	}

	// Build pattern insights using the pattern detection service
	patternInsights := l.svcCtx.PatternDetection.AnalyzeLite(recentCheckIns, habits, userLoc)

	// Add habit count for backward compatibility
	if len(habits) > 0 {
		patternInsights["habit_count"] = strconv.Itoa(len(habits))
	}

	// Update context refresh timestamp if forced
	if in.ForceRefresh {
		_, err = l.svcCtx.Repo.CoachingProfiles.UpdateCoachingProfileContextRefresh(l.ctx, userID)
		if err != nil {
			l.Infof("failed to update context refresh timestamp: %v", err)
		}
	}

	// Build response
	protoProfile := dbCoachingProfileToProto(profile)
	protoGoals := make([]*client.Goal, len(goals))
	for i, goal := range goals {
		protoGoals[i] = dbGoalToProto(goal)
	}

	protoHabits := make([]*client.Habit, len(habits))
	for i, habit := range habits {
		protoHabits[i] = dbHabitToProto(habit)
	}

	protoCheckIns := make([]*client.CheckIn, len(recentCheckIns))
	for i, checkIn := range recentCheckIns {
		protoCheckIns[i] = dbCheckInToProto(checkIn)
	}

	protoSuggestions := make([]*client.PlanAdjustmentSuggestion, len(pendingSuggestions))
	for i, suggestion := range pendingSuggestions {
		protoSuggestions[i] = dbPlanAdjustmentSuggestionToProto(suggestion)
	}

	var protoWeeklyReview *client.WeeklyReview
	if len(latestWeeklyReview) > 0 && latestWeeklyReview[0].ID != uuid.Nil {
		protoWeeklyReview = dbWeeklyReviewToProto(latestWeeklyReview[0])
	}

	return &client.GetPersonalizationContextResponse{
		Context: &client.PersonalizationContext{
			Profile:            protoProfile,
			ActiveGoals:        protoGoals,
			ActiveHabits:       protoHabits,
			RecentCheckIns:     protoCheckIns,
			LatestWeeklyReview: protoWeeklyReview,
			PendingSuggestions: protoSuggestions,
			PatternInsights:    patternInsights,
		},
	}, nil
}

func dbGoalToProto(goal db.Goal) *client.Goal {
	return &client.Goal{
		Id:          goal.ID.String(),
		Title:       goal.Title,
		Description: goal.Description.String,
		Category:    goal.Category,
		DueDate:     goal.DueDate.Time.Unix(),
		Progress:    goal.Progress.Int32,
		Completed:   goal.Completed.Bool,
		UserId:      goal.UserID.String(),
		CreatedAt:   goal.CreatedAt.Unix(),
		UpdatedAt:   goal.UpdatedAt.Unix(),
	}
}

func dbHabitToProto(habit db.Habit) *client.Habit {
	return &client.Habit{
		Id:          habit.ID.String(),
		Name:        habit.Name,
		Description: habit.Description.String,
		Streak:      habit.Streak.Int32,
		Completed:   habit.Completed.Bool,
		Category:    habit.Category,
		UserId:      habit.UserID.String(),
		CreatedAt:   habit.CreatedAt.Unix(),
		UpdatedAt:   habit.UpdatedAt.Unix(),
	}
}

func dbCheckInToProto(checkIn db.CheckIn) *client.CheckIn {
	return &client.CheckIn{
		Id:        checkIn.ID.String(),
		UserId:    checkIn.UserID.String(),
		HabitId:   checkIn.HabitID.String(),
		Status:    checkIn.Status,
		Mood:      checkIn.Mood.String,
		Energy:    checkIn.Energy.String,
		Blocker:   checkIn.Blocker.String,
		Note:      checkIn.Note.String,
		CreatedAt: checkIn.CreatedAt.Unix(),
	}
}

func dbPlanAdjustmentSuggestionToProto(suggestion db.PlanAdjustmentSuggestion) *client.PlanAdjustmentSuggestion {
	var goalID, habitID string
	if suggestion.GoalID.Valid {
		goalID = suggestion.GoalID.UUID.String()
	}
	if suggestion.HabitID.Valid {
		habitID = suggestion.HabitID.UUID.String()
	}

	metadataJson := "{}"
	if suggestion.Metadata != nil {
		metadataJson = string(suggestion.Metadata)
	}

	return &client.PlanAdjustmentSuggestion{
		Id:             suggestion.ID.String(),
		UserId:         suggestion.UserID.String(),
		GoalId:         goalID,
		HabitId:        habitID,
		Source:         suggestion.Source,
		AdjustmentType: suggestion.AdjustmentType,
		Reason:         suggestion.Reason,
		Suggestion:     suggestion.Suggestion,
		Status:         suggestion.Status,
		MetadataJson:   metadataJson,
		CreatedAt:      suggestion.CreatedAt.Unix(),
		UpdatedAt:      suggestion.UpdatedAt.Unix(),
	}
}

func dbWeeklyReviewToProto(review db.WeeklyReview) *client.WeeklyReview {
	// Parse completion rate from string to double
	completionRate := 0.0
	if review.CompletionRate != "" {
		if _, err := fmt.Sscanf(review.CompletionRate, "%f", &completionRate); err != nil {
			completionRate = 0.0
		}
	}

	// Parse habit breakdown from JSON
	var habitBreakdown []*client.WeeklyReviewHabitBreakdown
	if len(review.HabitBreakdown) > 0 {
		if err := json.Unmarshal(review.HabitBreakdown, &habitBreakdown); err != nil {
			logx.Errorf("failed to unmarshal habit breakdown: %v", err)
		}
	}

	// Parse suggested adjustments from JSON
	var suggestedAdjustments []*client.WeeklyReviewAdjustment
	if len(review.SuggestedAdjustments) > 0 {
		if err := json.Unmarshal(review.SuggestedAdjustments, &suggestedAdjustments); err != nil {
			logx.Errorf("failed to unmarshal suggested adjustments: %v", err)
		}
	}

	// Parse next week plan from JSON
	var nextWeekPlan *client.WeeklyReviewNextWeekPlan
	if len(review.NextWeekPlan) > 0 {
		if err := json.Unmarshal(review.NextWeekPlan, &nextWeekPlan); err != nil {
			logx.Errorf("failed to unmarshal next week plan: %v", err)
		}
	}

	// Parse mood summary from JSON
	var moodSummary map[string]int32
	if len(review.MoodSummary) > 0 {
		if err := json.Unmarshal(review.MoodSummary, &moodSummary); err != nil {
			logx.Errorf("failed to unmarshal mood summary: %v", err)
		}
	}

	// Parse energy summary from JSON
	var energySummary map[string]int32
	if len(review.EnergySummary) > 0 {
		if err := json.Unmarshal(review.EnergySummary, &energySummary); err != nil {
			logx.Errorf("failed to unmarshal energy summary: %v", err)
		}
	}

	return &client.WeeklyReview{
		Id:                   review.ID.String(),
		UserId:               review.UserID.String(),
		WeekStart:            review.WeekStart.Format("2006-01-02"),
		WeekEnd:              review.WeekEnd.Format("2006-01-02"),
		TotalHabits:          review.TotalHabits,
		CompletedCheckIns:    review.CompletedCheckIns,
		MissedCheckIns:       review.MissedCheckIns,
		CompletionRate:       completionRate,
		BestDay:              review.BestDay.String,
		HardestDay:           review.HardestDay.String,
		TopBlocker:           review.TopBlocker.String,
		MoodSummary:          moodSummary,
		EnergySummary:        energySummary,
		HabitBreakdown:       habitBreakdown,
		AiSummary:            review.AiSummary.String,
		SuggestedAdjustments: suggestedAdjustments,
		NextWeekPlan:         nextWeekPlan,
		GeneratedAt:          review.GeneratedAt.Unix(),
	}
}
