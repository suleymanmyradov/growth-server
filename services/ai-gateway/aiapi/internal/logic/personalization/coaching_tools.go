package personalization

import (
	"context"
	"fmt"
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	clientcheckin "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/checkinservice"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"
	clienthabits "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habits"
	clientpersonalization "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/personalizationservice"
	clientweekly "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/weeklyreviewservice"
)

// CoachingToolDeps wraps the RPC clients the coaching tools need. Tools
// that require an explicit user ID (check-ins, weekly reviews, suggestions,
// coaching profile) close over it; goals and habits are scoped via the
// propagated JWT in gRPC metadata.
type CoachingToolDeps struct {
	Goals           clientgoals.Goals
	Habits          clienthabits.Habits
	CheckIns        clientcheckin.CheckInService
	WeeklyReviews   clientweekly.WeeklyReviewService
	Personalization clientpersonalization.PersonalizationService
}

// BuildCoachingTools creates the set of on-demand retrieval tools for the
// agentic coaching flow. Each tool fetches a specific slice of user data
// from the client RPC only when the model decides it's needed.
//
// The tools return lean, model-shaped summaries — not full DB rows — to
// keep token cost down even when a tool is called.
//
// userID is needed for tools that call RPCs requiring an explicit user ID
// (check-ins, weekly reviews, suggestions, coaching profile). Goals and
// habits are scoped via the propagated JWT in the gRPC metadata, so their
// tools don't use userID directly.
func BuildCoachingTools(userID string, deps CoachingToolDeps) []ai.Tool {
	return []ai.Tool{
		getActiveGoalsTool(deps.Goals),
		getActiveHabitsTool(deps.Habits),
		getRecentCheckInsTool(userID, deps.CheckIns),
		getLatestWeeklyReviewTool(userID, deps.WeeklyReviews),
		getPendingSuggestionsTool(userID, deps.Personalization),
		getCoachingProfileTool(userID, deps.Personalization),
	}
}

// --- Tool input/output types (lean, model-shaped) ---

type noInput struct{}

type goalSummary struct {
	Title     string `json:"title"`
	Category  string `json:"category,omitempty"`
	Progress  int32  `json:"progress,omitempty"`
	Completed bool   `json:"completed,omitempty"`
	DueDate   string `json:"dueDate,omitempty"`
}

type goalsOutput struct {
	Goals []goalSummary `json:"goals"`
}

type habitSummary struct {
	Name           string `json:"name"`
	Category       string `json:"category,omitempty"`
	Streak         int32  `json:"streak,omitempty"`
	CompletedToday bool   `json:"completedToday,omitempty"`
}

type habitsOutput struct {
	Habits []habitSummary `json:"habits"`
}

type checkInSummary struct {
	Status  string `json:"status"`
	Mood    string `json:"mood,omitempty"`
	Energy  string `json:"energy,omitempty"`
	Blocker string `json:"blocker,omitempty"`
	Note    string `json:"note,omitempty"`
	Date    string `json:"date,omitempty"`
}

type checkInsOutput struct {
	CheckIns []checkInSummary `json:"checkIns"`
}

type weeklyReviewSummary struct {
	WeekStart      string `json:"weekStart"`
	WeekEnd        string `json:"weekEnd"`
	CompletionRate string `json:"completionRate,omitempty"`
	TopBlocker     string `json:"topBlocker,omitempty"`
	BestDay        string `json:"bestDay,omitempty"`
	HardestDay     string `json:"hardestDay,omitempty"`
	AISummary      string `json:"aiSummary,omitempty"`
}

type weeklyReviewOutput struct {
	Review *weeklyReviewSummary `json:"review,omitempty"`
}

type suggestionSummary struct {
	AdjustmentType string `json:"adjustmentType"`
	Reason         string `json:"reason,omitempty"`
	Suggestion     string `json:"suggestion,omitempty"`
	Status         string `json:"status,omitempty"`
}

type suggestionsOutput struct {
	Suggestions []suggestionSummary `json:"suggestions"`
}

type coachingProfileSummary struct {
	AccountabilityStyle  string   `json:"accountabilityStyle,omitempty"`
	PreferredTone        string   `json:"preferredTone,omitempty"`
	DifficultyPreference string   `json:"difficultyPreference,omitempty"`
	CommonBlockers       []string `json:"commonBlockers,omitempty"`
}

type coachingProfileOutput struct {
	Profile *coachingProfileSummary `json:"profile,omitempty"`
}

// --- Tool implementations ---

func getActiveGoalsTool(goals clientgoals.Goals) ai.Tool {
	return ai.NewTool[noInput, goalsOutput](ai.ToolSpec{
		Name:        "get_active_goals",
		Description: "Fetch the user's active goals with title, category, progress, and due date. Call this when the user asks about goals, progress, priorities, or what they're working toward.",
		Handler: func(ctx context.Context, _ noInput) (goalsOutput, error) {
			resp, err := goals.ListGoals(ctx, &clientgoals.ListGoalsRequest{
				Page:  1,
				Limit: 50,
			})
			if err != nil {
				return goalsOutput{}, fmt.Errorf("get_active_goals: %w", err)
			}
			out := goalsOutput{Goals: make([]goalSummary, 0, len(resp.Goals))}
			for _, g := range resp.Goals {
				summary := goalSummary{
					Title:     g.Title,
					Category:  g.Category,
					Progress:  g.Progress,
					Completed: g.Completed,
				}
				if g.DueDate > 0 {
					summary.DueDate = time.Unix(g.DueDate, 0).Format("2006-01-02")
				}
				out.Goals = append(out.Goals, summary)
			}
			return out, nil
		},
	})
}

func getActiveHabitsTool(habits clienthabits.Habits) ai.Tool {
	return ai.NewTool[noInput, habitsOutput](ai.ToolSpec{
		Name:        "get_active_habits",
		Description: "Fetch the user's active habits with name, category, current streak, and whether completed today. Call this when the user asks about habits, routines, streaks, or daily practices.",
		Handler: func(ctx context.Context, _ noInput) (habitsOutput, error) {
			resp, err := habits.ListHabits(ctx, &clienthabits.ListHabitsRequest{
				Page:  1,
				Limit: 50,
			})
			if err != nil {
				return habitsOutput{}, fmt.Errorf("get_active_habits: %w", err)
			}
			out := habitsOutput{Habits: make([]habitSummary, 0, len(resp.Habits))}
			for _, h := range resp.Habits {
				out.Habits = append(out.Habits, habitSummary{
					Name:           h.Name,
					Category:       h.Category,
					Streak:         h.Streak,
					CompletedToday: h.CompletedToday,
				})
			}
			return out, nil
		},
	})
}

func getRecentCheckInsTool(userID string, checkIns clientcheckin.CheckInService) ai.Tool {
	return ai.NewTool[noInput, checkInsOutput](ai.ToolSpec{
		Name:        "get_recent_check_ins",
		Description: "Fetch the user's recent check-ins (last 30 days, up to 50) with status, mood, energy, blocker, and note. Call this when the user asks about recent progress, struggles, patterns, or how they've been doing.",
		Handler: func(ctx context.Context, _ noInput) (checkInsOutput, error) {
			resp, err := checkIns.GetCheckInHistory(ctx, &clientcheckin.GetCheckInHistoryRequest{
				UserId: userID,
				Page:   1,
				Limit:  50,
			})
			if err != nil {
				return checkInsOutput{}, fmt.Errorf("get_recent_check_ins: %w", err)
			}
			out := checkInsOutput{CheckIns: make([]checkInSummary, 0, len(resp.CheckIns))}
			for _, c := range resp.CheckIns {
				summary := checkInSummary{
					Status:  c.Status,
					Mood:    c.Mood,
					Energy:  c.Energy,
					Blocker: c.Blocker,
					Note:    c.Note,
				}
				if c.CreatedAt > 0 {
					summary.Date = time.Unix(c.CreatedAt, 0).Format("2006-01-02")
				}
				out.CheckIns = append(out.CheckIns, summary)
			}
			return out, nil
		},
	})
}

func getLatestWeeklyReviewTool(userID string, weeklyReviews clientweekly.WeeklyReviewService) ai.Tool {
	return ai.NewTool[noInput, weeklyReviewOutput](ai.ToolSpec{
		Name:        "get_latest_weekly_review",
		Description: "Fetch the user's most recent weekly review with completion rate, top blocker, best/hardest day, and AI summary. Call this when the user asks about weekly performance, trends, or reflections.",
		Handler: func(ctx context.Context, _ noInput) (weeklyReviewOutput, error) {
			resp, err := weeklyReviews.ListWeeklyReviews(ctx, &clientweekly.ListWeeklyReviewsRequest{
				UserId: userID,
				Page:   1,
				Limit:  1,
			})
			if err != nil {
				return weeklyReviewOutput{}, fmt.Errorf("get_latest_weekly_review: %w", err)
			}
			if len(resp.Reviews) == 0 {
				return weeklyReviewOutput{}, nil
			}
			r := resp.Reviews[0]
			return weeklyReviewOutput{
				Review: &weeklyReviewSummary{
					WeekStart:      r.WeekStart,
					WeekEnd:        r.WeekEnd,
					CompletionRate: fmt.Sprintf("%.0f%%", r.CompletionRate),
					TopBlocker:     r.TopBlocker,
					BestDay:        r.BestDay,
					HardestDay:     r.HardestDay,
					AISummary:      r.AiSummary,
				},
			}, nil
		},
	})
}

func getPendingSuggestionsTool(userID string, personalization clientpersonalization.PersonalizationService) ai.Tool {
	return ai.NewTool[noInput, suggestionsOutput](ai.ToolSpec{
		Name:        "get_pending_suggestions",
		Description: "Fetch pending plan adjustment suggestions (e.g. reduce frequency, change difficulty) with reason and status. Call this when the user asks about plan changes, adjustments, or recommendations.",
		Handler: func(ctx context.Context, _ noInput) (suggestionsOutput, error) {
			resp, err := personalization.ListPendingPlanAdjustmentSuggestions(ctx, &clientpersonalization.ListPendingPlanAdjustmentSuggestionsRequest{
				UserId: userID,
				Limit:  20,
			})
			if err != nil {
				return suggestionsOutput{}, fmt.Errorf("get_pending_suggestions: %w", err)
			}
			out := suggestionsOutput{Suggestions: make([]suggestionSummary, 0, len(resp.Suggestions))}
			for _, s := range resp.Suggestions {
				out.Suggestions = append(out.Suggestions, suggestionSummary{
					AdjustmentType: s.AdjustmentType,
					Reason:         s.Reason,
					Suggestion:     s.Suggestion,
					Status:         s.Status,
				})
			}
			return out, nil
		},
	})
}

func getCoachingProfileTool(userID string, personalization clientpersonalization.PersonalizationService) ai.Tool {
	return ai.NewTool[noInput, coachingProfileOutput](ai.ToolSpec{
		Name:        "get_coaching_profile",
		Description: "Fetch the user's coaching preferences: accountability style, preferred tone, difficulty preference, and common blockers. Call this when you need to tailor your coaching approach or understand what motivates the user.",
		Handler: func(ctx context.Context, _ noInput) (coachingProfileOutput, error) {
			resp, err := personalization.GetCoachingProfile(ctx, &clientpersonalization.GetCoachingProfileRequest{
				UserId: userID,
			})
			if err != nil {
				return coachingProfileOutput{}, fmt.Errorf("get_coaching_profile: %w", err)
			}
			if resp.Profile == nil {
				return coachingProfileOutput{}, nil
			}
			p := resp.Profile
			return coachingProfileOutput{
				Profile: &coachingProfileSummary{
					AccountabilityStyle:  p.AccountabilityStyle,
					PreferredTone:        p.PreferredTone,
					DifficultyPreference: p.DifficultyPreference,
					CommonBlockers:       p.CommonBlockers,
				},
			}, nil
		},
	})
}
