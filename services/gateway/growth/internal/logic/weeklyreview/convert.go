package weeklyreview

import (
	"time"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	pbclient "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func protoToWeeklyReview(r *pbclient.WeeklyReview) types.WeeklyReview {
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
	var nextWeekPlan types.WeeklyReviewNextWeekPlan
	if plan != nil {
		nextWeekPlan = types.WeeklyReviewNextWeekPlan{
			Focus:           plan.Focus,
			Commitments:     plan.Commitments,
			Risks:           plan.Risks,
			RecoveryActions: plan.RecoveryActions,
		}
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
		MoodSummary:          r.MoodSummary,
		EnergySummary:        r.EnergySummary,
		HabitBreakdown:       habits,
		AiSummary:            r.AiSummary,
		SuggestedAdjustments: adjustments,
		NextWeekPlan:         nextWeekPlan,
		GeneratedAt:          formatTime(r.GeneratedAt),
	}
}

func formatTime(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).Format(time.RFC3339)
}
