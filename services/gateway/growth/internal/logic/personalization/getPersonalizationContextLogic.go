// Code scaffolded by goctl. Safe to edit.
// goctl 1.10.1

package personalization

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	clientpersonalization "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/personalizationservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/zeromicro/go-zero/core/logx"
)

type GetPersonalizationContextLogic struct {
	logx.Logger
	ctx    context.Context
	svcCtx *svc.ServiceContext
}

func NewGetPersonalizationContextLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPersonalizationContextLogic {
	return &GetPersonalizationContextLogic{
		Logger: logx.WithContext(ctx),
		ctx:    ctx,
		svcCtx: svcCtx,
	}
}

func (l *GetPersonalizationContextLogic) GetPersonalizationContext(req *types.GetPersonalizationContextRequest) (resp *types.PersonalizationContextResponse, err error) {
	principal, ok := principal.PrincipalFrom(l.ctx)
	if !ok {
		return nil, errors.New("unauthorized")
	}

	rpcResp, err := l.svcCtx.PersonalizationRpc.GetPersonalizationContext(l.ctx, &clientpersonalization.GetPersonalizationContextRequest{
		UserId:       principal.UserID,
		ForceRefresh: req.ForceRefresh,
	})
	if err != nil {
		return nil, err
	}

	// Parse coaching notes JSON
	var coachingNotes map[string]string
	if rpcResp.Context.Profile.CoachingNotesJson != "" {
		if err := json.Unmarshal([]byte(rpcResp.Context.Profile.CoachingNotesJson), &coachingNotes); err != nil {
			coachingNotes = make(map[string]string)
		}
	} else {
		coachingNotes = make(map[string]string)
	}

	// Convert RPC response to API response
	profile := types.CoachingProfile{
		Id:                   rpcResp.Context.Profile.Id,
		UserId:               rpcResp.Context.Profile.UserId,
		AccountabilityStyle:  rpcResp.Context.Profile.AccountabilityStyle,
		PreferredTone:        rpcResp.Context.Profile.PreferredTone,
		DifficultyPreference: rpcResp.Context.Profile.DifficultyPreference,
		PrimaryMotivation:    rpcResp.Context.Profile.PrimaryMotivation,
		CommonBlockers:       rpcResp.Context.Profile.CommonBlockers,
		CoachingNotes:        coachingNotes,
		LastContextRefreshAt: formatTimestamp(rpcResp.Context.Profile.LastContextRefreshAt),
		CreatedAt:            formatTimestamp(rpcResp.Context.Profile.CreatedAt),
		UpdatedAt:            formatTimestamp(rpcResp.Context.Profile.UpdatedAt),
	}

	activeGoals := make([]types.Goal, len(rpcResp.Context.ActiveGoals))
	for i, goal := range rpcResp.Context.ActiveGoals {
		activeGoals[i] = types.Goal{
			Id:          goal.Id,
			Title:       goal.Title,
			Description: goal.Description,
			Category:    goal.Category,
			DueDate:     formatTimestamp(goal.DueDate),
			Progress:    int(goal.Progress),
			Completed:   goal.Completed,
			UserId:      goal.UserId,
			CreatedAt:   formatTimestamp(goal.CreatedAt),
			UpdatedAt:   formatTimestamp(goal.UpdatedAt),
		}
	}

	activeHabits := make([]types.Habit, len(rpcResp.Context.ActiveHabits))
	for i, habit := range rpcResp.Context.ActiveHabits {
		activeHabits[i] = types.Habit{
			Id:          habit.Id,
			Name:        habit.Name,
			Description: habit.Description,
			Streak:      int(habit.Streak),
			Completed:   habit.Completed,
			Category:    habit.Category,
			UserId:      habit.UserId,
			CreatedAt:   formatTimestamp(habit.CreatedAt),
			UpdatedAt:   formatTimestamp(habit.UpdatedAt),
		}
	}

	recentCheckIns := make([]types.CheckIn, len(rpcResp.Context.RecentCheckIns))
	for i, checkIn := range rpcResp.Context.RecentCheckIns {
		recentCheckIns[i] = types.CheckIn{
			Id:        checkIn.Id,
			UserId:    checkIn.UserId,
			HabitId:   checkIn.HabitId,
			Status:    checkIn.Status,
			Mood:      checkIn.Mood,
			Energy:    checkIn.Energy,
			Blocker:   checkIn.Blocker,
			Note:      checkIn.Note,
			CreatedAt: formatTimestamp(checkIn.CreatedAt),
		}
	}

	var latestWeeklyReview types.WeeklyReview
	if rpcResp.Context.LatestWeeklyReview != nil {
		latestWeeklyReview = types.WeeklyReview{
			Id:                   rpcResp.Context.LatestWeeklyReview.Id,
			UserId:               rpcResp.Context.LatestWeeklyReview.UserId,
			WeekStart:            rpcResp.Context.LatestWeeklyReview.WeekStart,
			WeekEnd:              rpcResp.Context.LatestWeeklyReview.WeekEnd,
			CompletionRate:       rpcResp.Context.LatestWeeklyReview.CompletionRate,
			TopBlocker:           rpcResp.Context.LatestWeeklyReview.TopBlocker,
			MoodSummary:          rpcResp.Context.LatestWeeklyReview.MoodSummary,
			EnergySummary:        rpcResp.Context.LatestWeeklyReview.EnergySummary,
			HabitBreakdown:       convertHabitBreakdown(rpcResp.Context.LatestWeeklyReview.HabitBreakdown),
			AiSummary:            rpcResp.Context.LatestWeeklyReview.AiSummary,
			SuggestedAdjustments: convertAdjustments(rpcResp.Context.LatestWeeklyReview.SuggestedAdjustments),
			NextWeekPlan:         convertNextWeekPlan(rpcResp.Context.LatestWeeklyReview.NextWeekPlan),
			GeneratedAt:          formatTimestamp(rpcResp.Context.LatestWeeklyReview.GeneratedAt),
		}
	}

	pendingSuggestions := make([]types.PlanAdjustmentSuggestion, len(rpcResp.Context.PendingSuggestions))
	for i, suggestion := range rpcResp.Context.PendingSuggestions {
		// Parse metadata JSON
		var metadata map[string]string
		if suggestion.MetadataJson != "" {
			if err := json.Unmarshal([]byte(suggestion.MetadataJson), &metadata); err != nil {
				metadata = make(map[string]string)
			}
		} else {
			metadata = make(map[string]string)
		}

		pendingSuggestions[i] = types.PlanAdjustmentSuggestion{
			Id:             suggestion.Id,
			UserId:         suggestion.UserId,
			AdjustmentType: suggestion.AdjustmentType,
			GoalId:         suggestion.GoalId,
			HabitId:        suggestion.HabitId,
			Source:         suggestion.Source,
			Reason:         suggestion.Reason,
			Suggestion:     suggestion.Suggestion,
			Metadata:       metadata,
			Status:         suggestion.Status,
			CreatedAt:      formatTimestamp(suggestion.CreatedAt),
			UpdatedAt:      formatTimestamp(suggestion.UpdatedAt),
		}
	}

	return &types.PersonalizationContextResponse{
		Data: types.PersonalizationContext{
			Profile:            profile,
			ActiveGoals:        activeGoals,
			ActiveHabits:       activeHabits,
			RecentCheckIns:     recentCheckIns,
			LatestWeeklyReview: latestWeeklyReview,
			PendingSuggestions: pendingSuggestions,
			PatternInsights:    rpcResp.Context.PatternInsights,
		},
	}, nil
}

func convertHabitBreakdown(breakdown []*client.WeeklyReviewHabitBreakdown) []types.WeeklyReviewHabitBreakdown {
	result := make([]types.WeeklyReviewHabitBreakdown, len(breakdown))
	for i, hb := range breakdown {
		result[i] = types.WeeklyReviewHabitBreakdown{
			HabitId:        hb.HabitId,
			HabitName:      hb.HabitName,
			Category:       hb.Category,
			TotalCheckIns:  int(hb.TotalCheckIns),
			CompletedCount: int(hb.CompletedCount),
			MissedCount:    int(hb.MissedCount),
			CompletionRate: hb.CompletionRate,
			LastCheckInAt:  formatTimestamp(hb.LastCheckInAt),
		}
	}
	return result
}

func convertAdjustments(adjustments []*client.WeeklyReviewAdjustment) []types.WeeklyReviewAdjustment {
	result := make([]types.WeeklyReviewAdjustment, len(adjustments))
	for i, adj := range adjustments {
		result[i] = types.WeeklyReviewAdjustment{
			HabitId:        adj.HabitId,
			HabitName:      adj.HabitName,
			AdjustmentType: adj.AdjustmentType,
			Reason:         adj.Reason,
			Suggestion:     adj.Suggestion,
		}
	}
	return result
}

func convertNextWeekPlan(plan *client.WeeklyReviewNextWeekPlan) types.WeeklyReviewNextWeekPlan {
	if plan == nil {
		return types.WeeklyReviewNextWeekPlan{}
	}
	return types.WeeklyReviewNextWeekPlan{
		Focus:           plan.Focus,
		Commitments:     plan.Commitments,
		Risks:           plan.Risks,
		RecoveryActions: plan.RecoveryActions,
	}
}
