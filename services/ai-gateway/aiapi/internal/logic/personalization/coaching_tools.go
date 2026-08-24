package personalization

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
	clientarticles "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/articles"
	clientcheckin "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/checkinservice"
	clientgoals "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/goals"
	clienthabits "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/habits"
	clientpersonalization "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/personalizationservice"
	clientweekly "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/client/weeklyreviewservice"
	clientpb "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	searchservice "github.com/suleymanmyradov/growth-server/services/microservices/search/rpc/searchservice"
)

// CoachingToolDeps wraps the RPC clients the coaching tools need. Tools
// that require an explicit user ID (check-ins, weekly reviews, suggestions,
// coaching profile) close over it; goals and habits are scoped via the
// propagated JWT in gRPC metadata.
//
// Search and Articles are used by the search_articles tool (article
// reference). Search may be nil — the tool then returns an error and the
// coach falls back to a text-only reply.
type CoachingToolDeps struct {
	Goals           clientgoals.Goals
	Habits          clienthabits.Habits
	CheckIns        clientcheckin.CheckInService
	WeeklyReviews   clientweekly.WeeklyReviewService
	Personalization clientpersonalization.PersonalizationService
	Search          searchservice.SearchService
	Articles        clientarticles.Articles
	// Memory is the ai-coach long-term memory search. Optional: when nil the
	// search_past_conversations tool is not registered at all, so the model is
	// never told about a capability it does not have.
	Memory aicoachservice.AICoachService
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
	tools := []ai.Tool{
		getActiveGoalsTool(deps.Goals),
		getActiveHabitsTool(deps.Habits),
		getGoalTool(deps.Goals),
		getHabitTool(deps.Habits),
		getRecentCheckInsTool(userID, deps.CheckIns, deps.Habits),
		getLatestWeeklyReviewTool(userID, deps.WeeklyReviews),
		getPendingSuggestionsTool(userID, deps.Personalization),
		getCoachingProfileTool(userID, deps.Personalization),
		// Proposal tools (non-mutating). The agent calls these to prepare a
		// goal/habit create/update/delete for the user to confirm in-chat.
		// The handler emits a "proposal" SSE event; the client renders a
		// confirm card and calls the existing CRUD endpoint on accept.
		proposeCreateGoalTool(),
		proposeUpdateGoalTool(),
		proposeDeleteGoalTool(),
		proposeCreateHabitTool(),
		proposeUpdateHabitTool(),
		proposeDeleteHabitTool(),
		// Article reference (read-only). Searches published articles by
		// topic so the coach can cite relevant reading in its reply.
		searchArticlesTool(deps.Search, deps.Articles),
	}

	// Long-term memory as an explicit tool rather than always-on injection.
	// Two reasons: the default prompt stays small, and a reply grounded in a
	// tool result has an auditable basis -- the coach can say what it looked up
	// instead of appearing to remember by magic.
	if deps.Memory != nil {
		tools = append(tools, searchPastConversationsTool(userID, deps.Memory))
	}
	return tools
}

// --- Tool input/output types (lean, model-shaped) ---

type noInput struct{}

type goalSummary struct {
	Id        string `json:"id"`
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
	Id             string `json:"id"`
	Name           string `json:"name"`
	Category       string `json:"category,omitempty"`
	Streak         int32  `json:"streak,omitempty"`
	CompletedToday bool   `json:"completedToday,omitempty"`
}

type habitsOutput struct {
	Habits []habitSummary `json:"habits"`
}

type checkInsInput struct {
	HabitIds []string `json:"habitIds,omitempty"`
}

type checkInSummary struct {
	HabitId   string `json:"habitId"`
	HabitName string `json:"habitName,omitempty"`
	Status    string `json:"status"`
	Mood      string `json:"mood,omitempty"`
	Energy    string `json:"energy,omitempty"`
	Blocker   string `json:"blocker,omitempty"`
	Note      string `json:"note,omitempty"`
	Date      string `json:"date,omitempty"`
}

type dailyCoverageEntry struct {
	DayName   string `json:"dayName"`
	Date      string `json:"date"`
	Completed int    `json:"completed"`
	Missed    int    `json:"missed"`
	Missing   int    `json:"missing"`
	Expected  int    `json:"expected"`
}

type checkInsOutput struct {
	CheckIns       []checkInSummary     `json:"checkIns"`
	DailyCoverage  []dailyCoverageEntry `json:"dailyCoverage,omitempty"`
	CompletionRate float64              `json:"completionRate"`
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
		Description: "Fetch the user's active goals (up to 10) with id, title, category, progress, and due date. Call this when the user asks about goals, progress, priorities, or what they're working toward. Returns a summary list — use get_goal to fetch full details (description, milestones, measurement) for a specific goal when needed.",
		Handler: func(ctx context.Context, _ noInput) (goalsOutput, error) {
			resp, err := goals.ListGoals(ctx, &clientgoals.ListGoalsRequest{
				Page:  1,
				Limit: 10,
			})
			if err != nil {
				return goalsOutput{}, fmt.Errorf("get_active_goals: %w", err)
			}
			out := goalsOutput{Goals: make([]goalSummary, 0, len(resp.Goals))}
			for _, g := range resp.Goals {
				summary := goalSummary{
					Id:        g.Id,
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
		Description: "Fetch the user's active habits (up to 10) with id, name, category, current streak, and whether completed today. Call this when the user asks about habits, routines, streaks, or daily practices. Returns a summary list — use get_habit to fetch full details (description, recent history) for a specific habit when needed.",
		Handler: func(ctx context.Context, _ noInput) (habitsOutput, error) {
			resp, err := habits.ListHabits(ctx, &clienthabits.ListHabitsRequest{
				Page:  1,
				Limit: 10,
			})
			if err != nil {
				return habitsOutput{}, fmt.Errorf("get_active_habits: %w", err)
			}
			out := habitsOutput{Habits: make([]habitSummary, 0, len(resp.Habits))}
			for _, h := range resp.Habits {
				out.Habits = append(out.Habits, habitSummary{
					Id:             h.Id,
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

// --- Detail types for get_goal / get_habit ---

type milestoneDetail struct {
	Id        string `json:"id,omitempty"`
	Title     string `json:"title"`
	SortOrder int32  `json:"sortOrder,omitempty"`
	Completed bool   `json:"completed,omitempty"`
}

type goalDetail struct {
	Id              string            `json:"id"`
	Title           string            `json:"title"`
	Description     string            `json:"description,omitempty"`
	Category        string            `json:"category,omitempty"`
	Progress        int32             `json:"progress,omitempty"`
	Completed       bool              `json:"completed,omitempty"`
	DueDate         string            `json:"dueDate,omitempty"`
	Measurement     string            `json:"measurement,omitempty"`
	StartValue      float64           `json:"startValue,omitempty"`
	CurrentValue    float64           `json:"currentValue,omitempty"`
	TargetValue     float64           `json:"targetValue,omitempty"`
	Unit            string            `json:"unit,omitempty"`
	RelatedHabitIds []string          `json:"relatedHabitIds,omitempty"`
	Milestones      []milestoneDetail `json:"milestones,omitempty"`
}

type goalDetailOutput struct {
	Goal *goalDetail `json:"goal,omitempty"`
}

type habitDetail struct {
	Id             string `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	Category       string `json:"category,omitempty"`
	Streak         int32  `json:"streak,omitempty"`
	CompletedToday bool   `json:"completedToday,omitempty"`
}

type habitDetailOutput struct {
	Habit *habitDetail `json:"habit,omitempty"`
}

// getGoalByIdInput is the input for the get_goal tool.
type getGoalByIdInput struct {
	GoalId string `json:"goalId"`
}

// getHabitByIdInput is the input for the get_habit tool.
type getHabitByIdInput struct {
	HabitId string `json:"habitId"`
}

// getGoalTool fetches a single goal by ID with full details (description,
// milestones, measurement, values). The model calls this after
// get_active_goals when it needs more than the summary list provides.
func getGoalTool(goals clientgoals.Goals) ai.Tool {
	return ai.NewTool[getGoalByIdInput, goalDetailOutput](ai.ToolSpec{
		Name:        "get_goal",
		Description: "Fetch a single goal by ID with full details: description, measurement type, start/current/target values, unit, related habit IDs, and milestones. Call this after get_active_goals when you need more detail about a specific goal (e.g. the user asks about a particular goal's milestones, progress values, or description). Required: goalId (from get_active_goals).",
		Handler: func(ctx context.Context, in getGoalByIdInput) (goalDetailOutput, error) {
			if in.GoalId == "" {
				return goalDetailOutput{}, fmt.Errorf("get_goal: goalId is required")
			}
			resp, err := goals.GetGoal(ctx, &clientgoals.GetGoalRequest{
				GoalId: in.GoalId,
			})
			if err != nil {
				return goalDetailOutput{}, fmt.Errorf("get_goal: %w", err)
			}
			if resp.Goal == nil {
				return goalDetailOutput{}, nil
			}
			g := resp.Goal
			detail := goalDetail{
				Id:           g.Id,
				Title:        g.Title,
				Description:  g.Description,
				Category:     g.Category,
				Progress:     g.Progress,
				Completed:    g.Completed,
				Measurement:  g.Measurement,
				StartValue:   g.StartValue,
				CurrentValue: g.CurrentValue,
				TargetValue:  g.TargetValue,
				Unit:         g.Unit,
			}
			if g.DueDate > 0 {
				detail.DueDate = time.Unix(g.DueDate, 0).Format("2006-01-02")
			}
			if len(g.RelatedHabitIds) > 0 {
				detail.RelatedHabitIds = g.RelatedHabitIds
			}
			if len(g.Milestones) > 0 {
				detail.Milestones = make([]milestoneDetail, 0, len(g.Milestones))
				for _, m := range g.Milestones {
					detail.Milestones = append(detail.Milestones, milestoneDetail{
						Id:        m.Id,
						Title:     m.Title,
						SortOrder: m.SortOrder,
						Completed: m.DoneAt > 0,
					})
				}
			}
			return goalDetailOutput{Goal: &detail}, nil
		},
	})
}

// getHabitTool fetches a single habit by ID with full details (description,
// streak, completion status). The model calls this after get_active_habits
// when it needs more than the summary list provides.
func getHabitTool(habits clienthabits.Habits) ai.Tool {
	return ai.NewTool[getHabitByIdInput, habitDetailOutput](ai.ToolSpec{
		Name:        "get_habit",
		Description: "Fetch a single habit by ID with full details: description, category, current streak, and whether completed today. Call this after get_active_habits when you need more detail about a specific habit (e.g. the user asks about a particular habit's description or details). Required: habitId (from get_active_habits).",
		Handler: func(ctx context.Context, in getHabitByIdInput) (habitDetailOutput, error) {
			if in.HabitId == "" {
				return habitDetailOutput{}, fmt.Errorf("get_habit: habitId is required")
			}
			resp, err := habits.GetHabit(ctx, &clienthabits.GetHabitRequest{
				HabitId: in.HabitId,
			})
			if err != nil {
				return habitDetailOutput{}, fmt.Errorf("get_habit: %w", err)
			}
			if resp.Habit == nil {
				return habitDetailOutput{}, nil
			}
			h := resp.Habit
			return habitDetailOutput{Habit: &habitDetail{
				Id:             h.Id,
				Name:           h.Name,
				Description:    h.Description,
				Category:       h.Category,
				Streak:         h.Streak,
				CompletedToday: h.CompletedToday,
			}}, nil
		},
	})
}

func getRecentCheckInsTool(userID string, checkIns clientcheckin.CheckInService, habits clienthabits.Habits) ai.Tool {
	return ai.NewTool[checkInsInput, checkInsOutput](ai.ToolSpec{
		Name:        "get_recent_check_ins",
		Description: "Fetch the user's recent check-ins with habit ID, habit name, status, mood, energy, blocker, and note. Optionally pass habitIds to focus on the habits linked to a specific goal. Also includes a day-by-day coverage summary for the last 7 days showing which days had check-ins and which were missed (no check-in logged at all). Call this when the user asks about recent progress, struggles, patterns, or how they've been doing.",
		Handler: func(ctx context.Context, in checkInsInput) (checkInsOutput, error) {
			habitIDs := make([]string, 0, len(in.HabitIds))
			seenHabitIDs := make(map[string]struct{}, len(in.HabitIds))
			for _, habitID := range in.HabitIds {
				if habitID == "" {
					continue
				}
				if _, ok := seenHabitIDs[habitID]; ok {
					continue
				}
				seenHabitIDs[habitID] = struct{}{}
				habitIDs = append(habitIDs, habitID)
				if len(habitIDs) == 10 {
					break
				}
			}

			var recentCheckIns []*clientpb.CheckIn
			if len(habitIDs) == 0 {
				resp, err := checkIns.GetCheckInHistory(ctx, &clientcheckin.GetCheckInHistoryRequest{
					UserId: userID,
					Page:   1,
					Limit:  10,
				})
				if err != nil {
					return checkInsOutput{}, fmt.Errorf("get_recent_check_ins: %w", err)
				}
				recentCheckIns = resp.CheckIns
			} else {
				for _, habitID := range habitIDs {
					resp, err := checkIns.GetCheckInHistory(ctx, &clientcheckin.GetCheckInHistoryRequest{
						UserId:  userID,
						HabitId: habitID,
						Page:    1,
						Limit:   10,
					})
					if err != nil {
						return checkInsOutput{}, fmt.Errorf("get_recent_check_ins for habit %s: %w", habitID, err)
					}
					recentCheckIns = append(recentCheckIns, resp.CheckIns...)
				}
				sort.Slice(recentCheckIns, func(i, j int) bool {
					return recentCheckIns[i].CreatedAt > recentCheckIns[j].CreatedAt
				})
				if len(recentCheckIns) > 30 {
					recentCheckIns = recentCheckIns[:30]
				}
			}

			// Build a daily coverage summary for the last 7 days so the coach
			// can see which days had no check-ins at all (gaps). Fetch the
			// active habits count to know the expected check-ins per day.
			habitsResp, hErr := habits.ListHabits(ctx, &clienthabits.ListHabitsRequest{
				Page:  1,
				Limit: 50,
			})
			activeHabitCount := len(habitIDs)
			habitNames := make(map[string]string)
			if hErr == nil {
				if activeHabitCount == 0 {
					activeHabitCount = len(habitsResp.Habits)
				}
				for _, habit := range habitsResp.Habits {
					habitNames[habit.Id] = habit.Name
				}
			}

			out := checkInsOutput{CheckIns: make([]checkInSummary, 0, len(recentCheckIns))}
			for _, c := range recentCheckIns {
				summary := checkInSummary{
					HabitId:   c.HabitId,
					HabitName: habitNames[c.HabitId],
					Status:    c.Status,
					Mood:      c.Mood,
					Energy:    c.Energy,
					Blocker:   c.Blocker,
					Note:      c.Note,
				}
				if c.CreatedAt > 0 {
					summary.Date = time.Unix(c.CreatedAt, 0).Format("2006-01-02")
				}
				out.CheckIns = append(out.CheckIns, summary)
			}

			// Group check-ins by date for the last 7 days.
			now := time.Now().UTC()
			sevenDaysAgo := now.AddDate(0, 0, -7)
			byDate := make(map[string]*dailyCoverageEntry, 7)
			var dateOrder []string
			for d := sevenDaysAgo; d.Before(now); d = d.AddDate(0, 0, 1) {
				dateKey := d.Format("2006-01-02")
				byDate[dateKey] = &dailyCoverageEntry{
					DayName:  d.Format("Monday"),
					Date:     dateKey,
					Expected: activeHabitCount,
				}
				dateOrder = append(dateOrder, dateKey)
			}
			completedTotal := 0
			expectedTotal := 0
			for _, c := range recentCheckIns {
				if c.CreatedAt == 0 {
					continue
				}
				dateKey := time.Unix(c.CreatedAt, 0).UTC().Format("2006-01-02")
				entry, ok := byDate[dateKey]
				if !ok {
					continue
				}
				switch c.Status {
				case "completed":
					entry.Completed++
				case "missed":
					entry.Missed++
				}
			}
			out.DailyCoverage = make([]dailyCoverageEntry, 0, len(dateOrder))
			for _, dateKey := range dateOrder {
				entry := byDate[dateKey]
				entry.Missing = entry.Expected - entry.Completed - entry.Missed
				if entry.Missing < 0 {
					entry.Missing = 0
				}
				completedTotal += entry.Completed
				expectedTotal += entry.Expected
				out.DailyCoverage = append(out.DailyCoverage, *entry)
			}
			if expectedTotal > 0 {
				out.CompletionRate = float64(completedTotal) / float64(expectedTotal) * 100
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
				Limit:  10,
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

// --- Proposal tools (non-mutating) ---
//
// These tools do NOT call any RPC. They validate the agent's input and
// return a proposalOutput describing the action the user can confirm
// in-chat. The handler emits a "proposal" SSE event from the tool result;
// the client renders a confirm card and calls the existing CRUD endpoint
// on accept. The payload shape matches the client's Create*/Update*
// request schemas (dueDate as ISO date string, etc.).

// proposalOutput is the shared output of every propose_* tool. The handler
// parses this from the tool result JSON to emit the SSE proposal event.
type proposalOutput struct {
	Id      string         `json:"id"`      // unique proposal id (uuid)
	Action  string         `json:"action"`  // create_goal|update_goal|delete_goal|create_habit|update_habit|delete_habit
	Payload map[string]any `json:"payload"` // fields the client CRUD function expects
}

// newProposalID generates a unique id for a proposal so the client can
// track it across confirm/cancel lifecycle.
func newProposalID() string { return uuid.NewString() }

// --- Goal proposal input types ---

// milestoneProposalInput mirrors the client's MilestoneInput: an optional id
// identifies an existing milestone to update (preserving its done_at), an
// empty id means create new. Used by propose_update_goal so the agent can
// reconcile milestones without losing completion state.
type milestoneProposalInput struct {
	Id    string `json:"id,omitempty"`
	Title string `json:"title"`
}

type createGoalInput struct {
	Title           string   `json:"title"`
	Description     string   `json:"description,omitempty"`
	Category        string   `json:"category"`
	DueDate         string   `json:"dueDate,omitempty"` // ISO date string (YYYY-MM-DD)
	RelatedHabitIds []string `json:"relatedHabitIds,omitempty"`
	// Measurement type. Defaults to "manual" (progress set by the user) when
	// empty. Use "binary" for done/not-done goals, "numeric" for a value
	// tracking toward a target (requires startValue + targetValue + unit),
	// "milestone" for a multi-step checklist (provide milestoneTitles), or
	// "habit" for progress derived from linked habits (provide relatedHabitIds).
	Measurement     string   `json:"measurement,omitempty"`
	StartValue      float64  `json:"startValue,omitempty"`
	CurrentValue    float64  `json:"currentValue,omitempty"`
	TargetValue     float64  `json:"targetValue,omitempty"`
	Unit            string   `json:"unit,omitempty"`
	MilestoneTitles []string `json:"milestoneTitles,omitempty"`
}

type updateGoalInput struct {
	GoalId          string   `json:"goalId"`
	Title           string   `json:"title,omitempty"`
	Description     string   `json:"description,omitempty"`
	Category        string   `json:"category,omitempty"`
	DueDate         string   `json:"dueDate,omitempty"`
	RelatedHabitIds []string `json:"relatedHabitIds,omitempty"`
	Measurement     string   `json:"measurement,omitempty"`
	StartValue      float64  `json:"startValue,omitempty"`
	CurrentValue    float64  `json:"currentValue,omitempty"`
	TargetValue     float64  `json:"targetValue,omitempty"`
	Unit            string   `json:"unit,omitempty"`
	// Milestones to reconcile: an entry with an id updates an existing
	// milestone (preserving done_at); an entry without an id creates a new
	// one. Milestones not in this list are deleted. Only meaningful when
	// measurement is "milestone".
	Milestones []milestoneProposalInput `json:"milestones,omitempty"`
}

type deleteGoalInput struct {
	GoalId string `json:"goalId"`
}

// --- Habit proposal input types ---

type createHabitInput struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category"`
}

type updateHabitInput struct {
	HabitId     string `json:"habitId"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category,omitempty"`
}

type deleteHabitInput struct {
	HabitId string `json:"habitId"`
}

// --- Goal proposal tool implementations ---

func proposeCreateGoalTool() ai.Tool {
	return ai.NewTool[createGoalInput, proposalOutput](ai.ToolSpec{
		Name: "propose_create_goal",
		Description: "Prepare a new goal for the user to confirm. Call this when the user asks to create or add a goal. The goal is NOT created yet — a confirmation card is shown to the user. Required: title, category. Optional: description, dueDate (YYYY-MM-DD), relatedHabitIds, measurement, startValue, currentValue, targetValue, unit, milestoneTitles. " +
			"measurement is one of: binary (done/not-done), numeric (track a value toward a target — also provide startValue, targetValue, and unit), milestone (multi-step checklist — also provide milestoneTitles), habit (progress derived from linked habits — also provide relatedHabitIds), manual (default — user sets progress themselves). Pick the measurement that best fits what the user describes.",
		Handler: func(ctx context.Context, in createGoalInput) (proposalOutput, error) {
			if in.Title == "" {
				return proposalOutput{}, fmt.Errorf("propose_create_goal: title is required")
			}
			payload := map[string]any{"title": in.Title}
			if in.Description != "" {
				payload["description"] = in.Description
			}
			if in.Category != "" {
				payload["category"] = in.Category
			}
			if in.DueDate != "" {
				payload["dueDate"] = in.DueDate
			}
			if len(in.RelatedHabitIds) > 0 {
				payload["relatedHabitIds"] = in.RelatedHabitIds
			}
			if in.Measurement != "" {
				payload["measurement"] = in.Measurement
			}
			if in.StartValue != 0 {
				payload["startValue"] = in.StartValue
			}
			if in.CurrentValue != 0 {
				payload["currentValue"] = in.CurrentValue
			}
			if in.TargetValue != 0 {
				payload["targetValue"] = in.TargetValue
			}
			if in.Unit != "" {
				payload["unit"] = in.Unit
			}
			if len(in.MilestoneTitles) > 0 {
				payload["milestoneTitles"] = in.MilestoneTitles
			}
			return proposalOutput{Id: newProposalID(), Action: "create_goal", Payload: payload}, nil
		},
	})
}

func proposeUpdateGoalTool() ai.Tool {
	return ai.NewTool[updateGoalInput, proposalOutput](ai.ToolSpec{
		Name: "propose_update_goal",
		Description: "Prepare changes to an existing goal for the user to confirm. Call this when the user asks to edit, rename, or change a goal. The goal is NOT updated yet — a confirmation card is shown. Required: goalId. Optional: title, description, category, dueDate, relatedHabitIds, measurement, startValue, currentValue, targetValue, unit, milestones. " +
			"measurement is one of: binary, numeric, milestone, habit, manual. Changing measurement is supported — provide the new measurement and any fields it requires (numeric: startValue/targetValue/unit; milestone: milestones; habit: relatedHabitIds). " +
			"milestones reconciles the milestone list: an entry with an id updates that existing milestone (preserving its completion), an entry without an id creates a new one, and milestones not listed are deleted. Only meaningful when measurement is milestone.",
		Handler: func(ctx context.Context, in updateGoalInput) (proposalOutput, error) {
			if in.GoalId == "" {
				return proposalOutput{}, fmt.Errorf("propose_update_goal: goalId is required")
			}
			payload := map[string]any{"goalId": in.GoalId}
			if in.Title != "" {
				payload["title"] = in.Title
			}
			if in.Description != "" {
				payload["description"] = in.Description
			}
			if in.Category != "" {
				payload["category"] = in.Category
			}
			if in.DueDate != "" {
				payload["dueDate"] = in.DueDate
			}
			if len(in.RelatedHabitIds) > 0 {
				payload["relatedHabitIds"] = in.RelatedHabitIds
			}
			if in.Measurement != "" {
				payload["measurement"] = in.Measurement
			}
			if in.StartValue != 0 {
				payload["startValue"] = in.StartValue
			}
			if in.CurrentValue != 0 {
				payload["currentValue"] = in.CurrentValue
			}
			if in.TargetValue != 0 {
				payload["targetValue"] = in.TargetValue
			}
			if in.Unit != "" {
				payload["unit"] = in.Unit
			}
			if len(in.Milestones) > 0 {
				ms := make([]map[string]any, 0, len(in.Milestones))
				for _, m := range in.Milestones {
					entry := map[string]any{"title": m.Title}
					if m.Id != "" {
						entry["id"] = m.Id
					}
					ms = append(ms, entry)
				}
				payload["milestones"] = ms
			}
			return proposalOutput{Id: newProposalID(), Action: "update_goal", Payload: payload}, nil
		},
	})
}

func proposeDeleteGoalTool() ai.Tool {
	return ai.NewTool[deleteGoalInput, proposalOutput](ai.ToolSpec{
		Name:        "propose_delete_goal",
		Description: "Prepare deletion of a goal for the user to confirm. Call this when the user asks to delete or remove a goal. The goal is NOT deleted yet — a confirmation card is shown. Required: goalId.",
		Handler: func(ctx context.Context, in deleteGoalInput) (proposalOutput, error) {
			if in.GoalId == "" {
				return proposalOutput{}, fmt.Errorf("propose_delete_goal: goalId is required")
			}
			return proposalOutput{
				Id:      newProposalID(),
				Action:  "delete_goal",
				Payload: map[string]any{"goalId": in.GoalId},
			}, nil
		},
	})
}

// --- Habit proposal tool implementations ---

func proposeCreateHabitTool() ai.Tool {
	return ai.NewTool[createHabitInput, proposalOutput](ai.ToolSpec{
		Name:        "propose_create_habit",
		Description: "Prepare a new habit for the user to confirm. Call this when the user asks to create or add a habit. The habit is NOT created yet — a confirmation card is shown. Required: name. Optional: description, category.",
		Handler: func(ctx context.Context, in createHabitInput) (proposalOutput, error) {
			if in.Name == "" {
				return proposalOutput{}, fmt.Errorf("propose_create_habit: name is required")
			}
			payload := map[string]any{"name": in.Name}
			if in.Description != "" {
				payload["description"] = in.Description
			}
			if in.Category != "" {
				payload["category"] = in.Category
			}
			return proposalOutput{Id: newProposalID(), Action: "create_habit", Payload: payload}, nil
		},
	})
}

func proposeUpdateHabitTool() ai.Tool {
	return ai.NewTool[updateHabitInput, proposalOutput](ai.ToolSpec{
		Name:        "propose_update_habit",
		Description: "Prepare changes to an existing habit for the user to confirm. Call this when the user asks to edit, rename, or change a habit. The habit is NOT updated yet — a confirmation card is shown. Required: habitId. At least one of: name, description, category.",
		Handler: func(ctx context.Context, in updateHabitInput) (proposalOutput, error) {
			if in.HabitId == "" {
				return proposalOutput{}, fmt.Errorf("propose_update_habit: habitId is required")
			}
			payload := map[string]any{"habitId": in.HabitId}
			if in.Name != "" {
				payload["name"] = in.Name
			}
			if in.Description != "" {
				payload["description"] = in.Description
			}
			if in.Category != "" {
				payload["category"] = in.Category
			}
			return proposalOutput{Id: newProposalID(), Action: "update_habit", Payload: payload}, nil
		},
	})
}

func proposeDeleteHabitTool() ai.Tool {
	return ai.NewTool[deleteHabitInput, proposalOutput](ai.ToolSpec{
		Name:        "propose_delete_habit",
		Description: "Prepare deletion of a habit for the user to confirm. Call this when the user asks to delete or remove a habit. The habit is NOT deleted yet — a confirmation card is shown. Required: habitId.",
		Handler: func(ctx context.Context, in deleteHabitInput) (proposalOutput, error) {
			if in.HabitId == "" {
				return proposalOutput{}, fmt.Errorf("propose_delete_habit: habitId is required")
			}
			return proposalOutput{
				Id:      newProposalID(),
				Action:  "delete_habit",
				Payload: map[string]any{"habitId": in.HabitId},
			}, nil
		},
	})
}

// --- Article search tool (read-only) ---

type searchArticlesInput struct {
	Query    string `json:"query"`
	Category string `json:"category,omitempty"`
	Limit    int32  `json:"limit,omitempty"`
}

type articleSummary struct {
	Id       string `json:"id"`
	Title    string `json:"title"`
	Summary  string `json:"summary,omitempty"`
	Url      string `json:"url,omitempty"`
	ReadTime int32  `json:"readTime,omitempty"`
	Category string `json:"category,omitempty"`
}

type articlesOutput struct {
	Articles []articleSummary `json:"articles"`
}

// searchArticlesTool searches published articles by topic via the search
// RPC, then hydrates the top results with full article metadata via the
// client Articles RPC. Returns lean summaries the coach can cite.
// If search is nil (not configured), returns an error so the coach falls
// back to a text-only reply.
func searchArticlesTool(search searchservice.SearchService, articles clientarticles.Articles) ai.Tool {
	return ai.NewTool[searchArticlesInput, articlesOutput](ai.ToolSpec{
		Name:        "search_articles",
		Description: "Search published articles by topic to recommend relevant reading. Call this when the user asks for articles, reading, resources, or references on a topic. Returns article titles, summaries, and IDs you can cite in your reply.",
		Handler: func(ctx context.Context, in searchArticlesInput) (articlesOutput, error) {
			if in.Query == "" {
				return articlesOutput{}, fmt.Errorf("search_articles: query is required")
			}
			if search == nil {
				return articlesOutput{}, fmt.Errorf("search_articles: search service not configured")
			}
			limit := in.Limit
			if limit <= 0 || limit > 10 {
				limit = 5
			}
			resp, err := search.Search(ctx, &searchservice.SearchRequest{
				Query: in.Query,
				// "article" (singular) is the type the search-sync indexer
				// writes; "articles" matched no documents.
				Types:  []string{"article"},
				Status: "published",
				Limit:  limit,
			})
			if err != nil {
				return articlesOutput{}, fmt.Errorf("search_articles: %w", err)
			}
			if len(resp.Results) == 0 {
				return articlesOutput{Articles: []articleSummary{}}, nil
			}

			// Hydrate via the client Articles RPC for full metadata (summary,
			// readTime, category). Fall back to search-result fields if
			// hydration fails or an article is missing.
			ids := make([]string, 0, len(resp.Results))
			for _, r := range resp.Results {
				if r.Id != "" {
					ids = append(ids, r.Id)
				}
			}

			out := articlesOutput{Articles: make([]articleSummary, 0, len(resp.Results))}
			// hydrated maps article ID → enriched summary (from the client
			// Articles RPC). We store pre-built articleSummary values so we
			// don't need to reference the generated Article proto type here.
			hydrated := make(map[string]articleSummary)
			if articles != nil && len(ids) > 0 {
				if hResp, hErr := articles.GetArticlesByIds(ctx, &clientarticles.GetArticlesByIdsRequest{Ids: ids}); hErr == nil && hResp != nil {
					for _, a := range hResp.Articles {
						s := articleSummary{
							Id:       a.Id,
							Title:    a.Title,
							Summary:  a.Summary,
							ReadTime: a.ReadTime,
						}
						if a.Category != nil {
							s.Category = a.Category.Name
						}
						hydrated[a.Id] = s
					}
				}
			}

			for _, r := range resp.Results {
				if h, ok := hydrated[r.Id]; ok {
					// Use the hydrated summary but keep the search-result URL
					// (the article proto doesn't carry a public URL).
					h.Url = r.Url
					out.Articles = append(out.Articles, h)
				} else {
					// Fall back to search-result fields only.
					out.Articles = append(out.Articles, articleSummary{
						Id:      r.Id,
						Title:   r.Title,
						Summary: r.Description,
						Url:     r.Url,
					})
				}
			}
			return out, nil
		},
	})
}

// --- Long-term memory ---

type searchPastConversationsInput struct {
	Query string `json:"query" jsonschema:"description=What to look for in the user's past conversations, check-ins, and weekly reviews"`
	Limit int32  `json:"limit,omitempty" jsonschema:"description=Maximum results to return (1-8, default 5)"`
}

type memoryHitSummary struct {
	Source string `json:"source"`
	When   string `json:"when,omitempty"`
	Text   string `json:"text"`
	Habit  string `json:"habit,omitempty"`
}

type memoryOutput struct {
	Hits []memoryHitSummary `json:"hits"`
}

// searchPastConversationsTool lets the coach look through the user's own
// history on demand instead of having snippets injected into every prompt.
//
// The description tells the model when NOT to call it as firmly as when to:
// recent turns are already in the message list, so searching for them wastes a
// round trip and returns the same text the model can already see.
func searchPastConversationsTool(userID string, memory aicoachservice.AICoachService) ai.Tool {
	return ai.NewTool[searchPastConversationsInput, memoryOutput](ai.ToolSpec{
		Name: "search_past_conversations",
		Description: "Search the user's own earlier conversations, check-in notes, and weekly reviews for something they mentioned before. " +
			"Call this when the user refers to something from the past that is not in the current conversation " +
			"(\"like I told you before\", \"the plan we made\", \"my usual routine\"), or when knowing what they said previously " +
			"would change your advice. Do NOT call it for anything already visible in this conversation, and do not call it speculatively — " +
			"an empty result is a real answer meaning they never mentioned it.",
		Handler: func(ctx context.Context, in searchPastConversationsInput) (memoryOutput, error) {
			if strings.TrimSpace(in.Query) == "" {
				return memoryOutput{}, fmt.Errorf("search_past_conversations: query is required")
			}
			if memory == nil {
				return memoryOutput{}, fmt.Errorf("search_past_conversations: long-term memory is not configured")
			}
			limit := in.Limit
			if limit <= 0 || limit > 8 {
				limit = 5
			}

			resp, err := memory.SearchMemory(ctx, &aicoachservice.SearchMemoryRequest{
				UserId: userID,
				Query:  in.Query,
				Limit:  limit,
			})
			if err != nil {
				return memoryOutput{}, fmt.Errorf("search_past_conversations: %w", err)
			}

			out := memoryOutput{Hits: make([]memoryHitSummary, 0, len(resp.Hits))}
			for _, h := range resp.Hits {
				if h.Content == "" {
					continue
				}
				hit := memoryHitSummary{
					Source: memorySourceLabel(h.EntityType, h.Role),
					Text:   h.Content,
					Habit:  h.HabitName,
				}
				if h.CreatedAt > 0 {
					hit.When = time.Unix(h.CreatedAt, 0).UTC().Format("2006-01-02")
				}
				out.Hits = append(out.Hits, hit)
			}
			return out, nil
		},
	})
}

// memorySourceLabel gives each hit a provenance tag, so the coach can attribute
// what it found ("in your check-in on the 3rd") instead of asserting it as
// something it simply knows.
func memorySourceLabel(entityType, role string) string {
	switch entityType {
	case "check_in":
		return "check-in note"
	case "weekly_review":
		return "weekly review"
	case "conversation_message":
		if role == "assistant" {
			return "your earlier reply"
		}
		return "what they said earlier"
	default:
		return entityType
	}
}
