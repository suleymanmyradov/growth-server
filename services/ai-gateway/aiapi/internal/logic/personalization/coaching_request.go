package personalization

import (
	aiprompts "github.com/suleymanmyradov/growth-server/pkg/ai/prompts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	clientpb "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

// buildPersonalizedCoachingRequest assembles the ai-coach RPC request from a
// personalization context (fetched from the client RPC) and the user's
// message + conversation history. This logic previously lived in the client
// RPC service; it was moved here so the client RPC no longer calls the
// ai-coach RPC (distributed-monolith rule: RPC services must not call other
// RPC services — only the gateway orchestrates cross-service flows).
func BuildPersonalizedCoachingRequest(
	userID, userMessage string,
	history []*clientpb.HistoryMessage,
	ctx *clientpb.PersonalizationContext,
) *aicoachservice.PersonalizedCoachingRequest {
	profile := ctx.Profile
	user := ctx.User

	activeGoals := make([]string, len(ctx.ActiveGoals))
	for i, goal := range ctx.ActiveGoals {
		activeGoals[i] = goal.Title
	}

	activeHabits := make([]string, len(ctx.ActiveHabits))
	for i, habit := range ctx.ActiveHabits {
		activeHabits[i] = habit.Name
	}

	// Calculate recent check-in summary. The completion rate must account
	// for days where the user did not log any check-in at all — those are
	// gaps, not passes. Expected check-ins = active habits × days in the
	// recent check-in window (last 30 days). Missing days count as missed.
	completedCount := 0
	for _, checkIn := range ctx.RecentCheckIns {
		if checkIn.Status == "completed" {
			completedCount++
		}
	}
	activeHabitCount := len(ctx.ActiveHabits)
	// The personalization context fetches check-ins for the last 30 days.
	const recentCheckInWindowDays = 30
	expectedCheckIns := activeHabitCount * recentCheckInWindowDays
	completionRate := 0.0
	if expectedCheckIns > 0 {
		completionRate = float64(completedCount) / float64(expectedCheckIns) * 100
		if completionRate > 100 {
			completionRate = 100
		}
	}

	patternInsights := make(map[string]string, len(ctx.PatternInsights))
	for k, v := range ctx.PatternInsights {
		patternInsights[k] = v
	}

	// Build an aggregate check-in digest (counts + trend + top blocker) instead
	// of enumerating raw check-in rows. Trend/top-blocker come from the
	// pre-computed pattern insights so this stays near-constant size.
	// Use expectedCheckIns (not len(RecentCheckIns)) so the count reflects the
	// true denominator — days with no check-in at all are gaps, not passes.
	checkInCountForSummary := expectedCheckIns
	if checkInCountForSummary == 0 {
		checkInCountForSummary = len(ctx.RecentCheckIns)
	}
	recentCheckInsSummary := aiprompts.BuildContextSummary(
		checkInCountForSummary,
		completionRate,
		patternInsights["top_blocker"],
		patternInsights["completion_pattern"],
	)

	// Convert history messages to the ai-coach proto type.
	aiHistory := make([]*aicoachservice.HistoryMessage, len(history))
	for i, h := range history {
		aiHistory[i] = &aicoachservice.HistoryMessage{
			Role:    h.Role,
			Content: h.Content,
		}
	}

	var commonBlockers []string
	if profile != nil {
		commonBlockers = profile.CommonBlockers
	}

	var userFullName, userBio, userLocation string
	var userInterests []string
	if user != nil {
		userFullName = user.FullName
		userBio = user.Bio
		userLocation = user.Location
		userInterests = user.Interests
	}

	var accountabilityStyle, preferredTone, difficultyPreference string
	if profile != nil {
		accountabilityStyle = profile.AccountabilityStyle
		preferredTone = profile.PreferredTone
		difficultyPreference = profile.DifficultyPreference
	}

	return &aicoachservice.PersonalizedCoachingRequest{
		UserId:                userID,
		UserMessage:           userMessage,
		AccountabilityStyle:   accountabilityStyle,
		PreferredTone:         preferredTone,
		DifficultyPreference:  difficultyPreference,
		ActiveGoals:           activeGoals,
		ActiveHabits:          activeHabits,
		RecentCheckInsSummary: recentCheckInsSummary,
		CommonBlockers:        commonBlockers,
		PatternInsights:       patternInsights,
		History:               aiHistory,
		UserFullName:          userFullName,
		UserBio:               userBio,
		UserLocation:          userLocation,
		UserInterests:         userInterests,
	}
}
