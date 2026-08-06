package weeklyreview

import (
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	clientpb "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

// BuildWeeklyReviewAIRequest converts the prepared weekly review data from
// the client RPC into an ai-coach WeeklyReviewRequest. This logic previously
// lived in the client RPC service; it was moved here so the client RPC no
// longer calls the ai-coach RPC (distributed-monolith rule).
func BuildWeeklyReviewAIRequest(data *clientpb.PreparedWeeklyReviewData) *aicoachservice.WeeklyReviewRequest {
	habitBreakdowns := make([]*aicoachservice.HabitBreakdown, len(data.HabitBreakdowns))
	for i, h := range data.HabitBreakdowns {
		habitBreakdowns[i] = &aicoachservice.HabitBreakdown{
			HabitId:        h.HabitId,
			HabitName:      h.HabitName,
			Category:       h.Category,
			CompletedCount: h.CompletedCount,
			MissedCount:    h.MissedCount,
			CompletionRate: float32(h.CompletionRate),
		}
	}

	blockerStats := make([]*aicoachservice.BlockerStat, len(data.BlockerStats))
	for i, b := range data.BlockerStats {
		blockerStats[i] = &aicoachservice.BlockerStat{
			Blocker: b.Blocker,
			Count:   b.Count,
		}
	}

	moodStats := make([]*aicoachservice.MoodStat, len(data.MoodStats))
	for i, m := range data.MoodStats {
		moodStats[i] = &aicoachservice.MoodStat{
			Mood:  m.Mood,
			Count: m.Count,
		}
	}

	energyStats := make([]*aicoachservice.EnergyStat, len(data.EnergyStats))
	for i, e := range data.EnergyStats {
		energyStats[i] = &aicoachservice.EnergyStat{
			Energy: e.Energy,
			Count:  e.Count,
		}
	}

	return &aicoachservice.WeeklyReviewRequest{
		UserId:               data.UserId,
		AccountabilityStyle:  data.AccountabilityStyle,
		PreferredTone:        data.PreferredTone,
		DifficultyPreference: data.DifficultyPreference,
		CommonBlockers:       data.CommonBlockers,
		Goals:                data.Goals,
		TotalHabits:          data.TotalHabits,
		CompletionRate:       float32(data.CompletionRate),
		CompletedCheckIns:    data.CompletedCheckIns,
		MissedCheckIns:       data.MissedCheckIns,
		BestDay:              data.BestDay,
		HardestDay:           data.HardestDay,
		TopBlocker:           data.TopBlocker,
		HabitBreakdowns:      habitBreakdowns,
		BlockerStats:         blockerStats,
		MoodStats:            moodStats,
		EnergyStats:          energyStats,
		DetectedPatterns:     data.DetectedPatterns,
	}
}

// convertAdjustmentsToClient converts ai-coach adjustment types to the client
// proto types for the SaveWeeklyReview RPC.
func ConvertAdjustmentsToClient(adjustments []*aicoachservice.WeeklyReviewAdjustment) []*clientpb.WeeklyReviewAdjustment {
	result := make([]*clientpb.WeeklyReviewAdjustment, 0, len(adjustments))
	for _, a := range adjustments {
		if a == nil {
			continue
		}
		result = append(result, &clientpb.WeeklyReviewAdjustment{
			HabitId:        a.HabitId,
			HabitName:      a.HabitName,
			Reason:         a.Reason,
			Suggestion:     a.Suggestion,
			AdjustmentType: a.AdjustmentType,
		})
	}
	return result
}

// convertNextWeekPlanToClient converts the ai-coach NextWeekPlan to the client
// proto type for the SaveWeeklyReview RPC.
func ConvertNextWeekPlanToClient(plan *aicoachservice.NextWeekPlan) *clientpb.WeeklyReviewNextWeekPlan {
	if plan == nil {
		return nil
	}
	return &clientpb.WeeklyReviewNextWeekPlan{
		Focus:           plan.Focus,
		Commitments:     plan.Commitments,
		Risks:           plan.Risks,
		RecoveryActions: plan.RecoveryActions,
	}
}
