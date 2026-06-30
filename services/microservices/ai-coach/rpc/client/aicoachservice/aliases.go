package aicoachservice

import "github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"

// Re-exports of nested proto message types that are referenced by callers but
// NOT generated as top-level aliases by `goctl rpc protoc` (goctl only aliases
// the direct request/response types of each RPC method).
//
// This file is hand-maintained and tracked in git so a clean
// `make generate-ai-coach-proto` does not break callers that reference these
// nested types via the aicoachservice package alias.
//
// If you add a new nested message to ai-coach.proto that callers need to
// reference by alias, add it here too.
type (
	HistoryMessage         = aicoach.HistoryMessage
	HabitBreakdown         = aicoach.HabitBreakdown
	BlockerStat            = aicoach.BlockerStat
	MoodStat               = aicoach.MoodStat
	EnergyStat             = aicoach.EnergyStat
	WeeklyReviewAdjustment = aicoach.WeeklyReviewAdjustment
	NextWeekPlan           = aicoach.NextWeekPlan
	OnboardingHabitSuggestion = aicoach.OnboardingHabitSuggestion
)
