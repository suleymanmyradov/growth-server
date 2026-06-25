package weeklyreview

import (
	"time"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	pbclient "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func ProtoToWeeklyReview(r *pbclient.WeeklyReview) types.WeeklyReview {
	habits := make([]types.WeeklyReviewHabitBreakdown, 0, len(r.HabitBreakdown))
	for _, h := range r.HabitBreakdown {
		habits = append(habits, types.WeeklyReviewHabitBreakdown{
			HabitId:        h.HabitId,
			HabitName:      h.HabitName,
			Category:       h.Category,
			TotalCheckIns:  int(h.TotalCheckIns),
			CompletedCount: int(h.CompletedCount),
			MissedCount:    int(h.MissedCount),
			CompletionRate: h.CompletionRate,
			LastCheckInAt:  formatTime(h.LastCheckInAt),
		})
	}

	adjustments := make([]types.WeeklyReviewAdjustment, 0, len(r.SuggestedAdjustments))
	for _, a := range r.SuggestedAdjustments {
		adjustments = append(adjustments, types.WeeklyReviewAdjustment{
			HabitId:        a.HabitId,
			HabitName:      a.HabitName,
			AdjustmentType: a.AdjustmentType,
			Reason:         a.Reason,
			Suggestion:     a.Suggestion,
		})
	}

	plan := r.NextWeekPlan
	nextWeekPlan := types.WeeklyReviewNextWeekPlan{
		Commitments:     []string{},
		Risks:           []string{},
		RecoveryActions: []string{},
	}
	if plan != nil {
		nextWeekPlan.Focus = plan.Focus
		if plan.Commitments != nil {
			nextWeekPlan.Commitments = plan.Commitments
		}
		if plan.Risks != nil {
			nextWeekPlan.Risks = plan.Risks
		}
		if plan.RecoveryActions != nil {
			nextWeekPlan.RecoveryActions = plan.RecoveryActions
		}
	}

	// gRPC delivers empty proto maps as nil; coerce to empty so the JSON
	// response stays consistent with the contract (objects, not null).
	mood := r.MoodSummary
	if mood == nil {
		mood = map[string]int32{}
	}
	energy := r.EnergySummary
	if energy == nil {
		energy = map[string]int32{}
	}

	return types.WeeklyReview{
		Id:                   r.Id,
		UserId:               r.UserId,
		WeekStart:            r.WeekStart,
		WeekEnd:              r.WeekEnd,
		TotalHabits:          int(r.TotalHabits),
		CompletedCheckIns:    int(r.CompletedCheckIns),
		MissedCheckIns:       int(r.MissedCheckIns),
		CompletionRate:       r.CompletionRate,
		BestDay:              r.BestDay,
		HardestDay:           r.HardestDay,
		TopBlocker:           r.TopBlocker,
		MoodSummary:          mood,
		EnergySummary:        energy,
		HabitBreakdown:       habits,
		AiSummary:            r.AiSummary,
		SuggestedAdjustments: adjustments,
		NextWeekPlan:         nextWeekPlan,
		GeneratedAt:          formatTime(r.GeneratedAt),
	}
}

// emptyWeeklyReview returns a well-formed zero-value review with non-nil
// collections, so the JSON response uses empty arrays/objects (not null)
// when the user has no review for the current week yet.
func emptyWeeklyReview() types.WeeklyReview {
	return types.WeeklyReview{
		MoodSummary:          map[string]int32{},
		EnergySummary:        map[string]int32{},
		HabitBreakdown:       []types.WeeklyReviewHabitBreakdown{},
		SuggestedAdjustments: []types.WeeklyReviewAdjustment{},
		NextWeekPlan: types.WeeklyReviewNextWeekPlan{
			Commitments:     []string{},
			Risks:           []string{},
			RecoveryActions: []string{},
		},
	}
}

func formatTime(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).Format(time.RFC3339)
}
