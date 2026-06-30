package prompts

import (
	"fmt"
	"strings"
)

// WeeklyReviewStructuredOutput holds the structured response from the LLM.
type WeeklyReviewStructuredOutput struct {
	AiSummary            string                   `json:"aiSummary"`
	SuggestedAdjustments []WeeklyReviewAdjustment `json:"suggestedAdjustments"`
	NextWeekPlan         WeeklyReviewNextWeekPlan `json:"nextWeekPlan"`
}

type WeeklyReviewAdjustment struct {
	HabitID        string `json:"habitId"`
	HabitName      string `json:"habitName"`
	Reason         string `json:"reason"`
	Suggestion     string `json:"suggestion"`
	AdjustmentType string `json:"adjustmentType"` // reduce_difficulty | change_time | clarify_plan | keep_same | pause
}

type WeeklyReviewNextWeekPlan struct {
	Focus           string   `json:"focus"`
	Commitments     []string `json:"commitments"`
	Risks           []string `json:"risks"`
	RecoveryActions []string `json:"recoveryActions"`
}

// WeeklyReviewInput holds the aggregate data compiled from database queries to feed into the prompt.
type WeeklyReviewInput struct {
	AccountabilityStyle  string
	PreferredTone        string
	DifficultyPreference string
	CommonBlockers       []string
	Goals                []string
	TotalHabits          int
	CompletionRate       float64
	CompletedCheckIns    int
	MissedCheckIns       int
	BestDay              string
	HardestDay           string
	TopBlocker           string
	HabitBreakdowns      []HabitBreakdownInput
	BlockerStats         []BlockerInput
	MoodStats            []MoodInput
	EnergyStats          []EnergyInput
	DetectedPatterns     []string
}

type HabitBreakdownInput struct {
	HabitID        string
	HabitName      string
	Category       string
	CompletedCount int
	MissedCount    int
	CompletionRate float64
}

type BlockerInput struct {
	Blocker string
	Count   int
}

type MoodInput struct {
	Mood  string
	Count int
}

type EnergyInput struct {
	Energy string
	Count  int
}

// BuildWeeklyReviewSystemPrompt returns the system instructions for the AI coach.
func BuildWeeklyReviewSystemPrompt(style string, preferredTone string, difficultyPreference string) string {
	var tone string
	switch style {
	case "gentle":
		tone = `You are a gentle, nurturing accountability coach. You validate struggles, celebrate small progress, avoid intense pressure, and provide a warm, encouraging environment to rebuild momentum.`
	case "strict":
		tone = `You are a strict, direct, no-nonsense accountability coach. You cut through excuses, highlight gaps in execution, and challenge the user to step up, while remaining constructive, professional, and respectful.`
	default: // "balanced"
		tone = `You are a balanced accountability coach. You offer constructive, objective feedback, celebrate clear wins, gently address challenges, and give practical, actionable suggestions.`
	}

	// Apply preferred tone customization
	switch preferredTone {
	case "supportive":
		tone += ` Your communication style is warm, empathetic, and encouraging. You focus on building confidence and celebrating progress.`
	case "direct":
		tone += ` Your communication style is straightforward, concise, and action-oriented. You focus on clear steps and measurable outcomes.`
	case "warm":
		tone += ` Your communication style is friendly, approachable, and conversational. You build rapport through relatable language.`
	case "practical":
		tone += ` Your communication style is pragmatic, solution-focused, and implementation-oriented. You emphasize actionable tactics over theory.`
	case "challenging":
		tone += ` Your communication style is motivating and pushes for growth. You challenge the user to exceed their current capabilities while remaining supportive.`
	}

	// Apply difficulty preference
	var difficultyGuidance string
	switch difficultyPreference {
	case "easy":
		difficultyGuidance = ` Suggest smaller, easily achievable steps. Focus on building momentum with minimal friction.`
	case "adaptive":
		difficultyGuidance = ` Adjust suggestions based on their current performance - easier when struggling, more challenging when excelling.`
	case "ambitious":
		difficultyGuidance = ` Suggest challenging but achievable goals that push for growth. Focus on rapid improvement and skill development.`
	default:
		difficultyGuidance = ` Provide balanced suggestions that match their current capability level.`
	}

	return tone + difficultyGuidance + `

Your task is to analyze the user's weekly habit performance and generate a structured weekly accountability review in JSON format.

You MUST respond with a valid JSON object matching the following structure:
{
  "aiSummary": "REQUIRED: A concise, engaging 3-4 sentence coaching analysis. Start with a warm greeting or dynamic opening. Synthesize their wins, pinpoint friction areas (like blockers, energy drops, or specific hard days), and provide a core encouraging theme. This field must be non-empty.",
  "suggestedAdjustments": [
    {
      "habitId": "uuid-of-habit",
      "habitName": "Name of Habit",
      "reason": "Why this adjustment is recommended based on the data (e.g. high missed rate, consistent blocker, or energy mismatches)",
      "suggestion": "Concrete, tactical suggestion (e.g., 'Do it at 8:00 AM instead of 6:00 PM', 'Scale down to 5 mins')",
      "adjustmentType": "reduce_difficulty"
    }
  ],
  "nextWeekPlan": {
    "focus": "A central theme/priority for next week (e.g. 'Build Consistent Morning Routine', 'Address Afternoon Energy Drops')",
    "commitments": ["2-3 specific, realistic habit commitments for next week"],
    "risks": ["1-2 potential blockers or risks predicted based on last week's data"],
    "recoveryActions": ["1-2 actionable recovery plans if they slip up or face a blocker"]
  }
}

Example of a valid response:
{"aiSummary": "This week you built strong momentum, finishing with an 85% completion rate. Your best day was Wednesday, and the only real friction was a scheduling conflict on Monday. Keep the current routine and protect your streak next week.", "suggestedAdjustments": [{"habitId": "0194e000-0000-7000-8000-000000000001", "habitName": "Morning meditation", "adjustmentType": "keep_same", "reason": "You maintained strong consistency.", "suggestion": "Continue your current 10-minute practice each morning."}], "nextWeekPlan": {"focus": "Protect the streak", "commitments": ["Meditate 10 minutes each morning", "Log the session before breakfast"], "risks": ["A busy Monday morning could break the streak"], "recoveryActions": ["If you miss the morning, do a 2-minute session before bed"]}}

You MUST include a non-empty aiSummary field. The output is invalid without it.

For each suggested adjustment, use the exact habitId and habitName from the Habit Breakdown section. Do not invent placeholder IDs like "habit-1" or generic names.

Rules for Suggested Adjustments:
- If a habit has low completion rate (e.g., < 50%), suggest adjustments like:
  - 'reduce_difficulty': Scale back volume or complexity.
  - 'change_time': Align habit to high energy periods or stable times.
  - 'clarify_plan': Redefine exact triggers or context.
  - 'pause_habit': Temporary pause if it's currently overwhelming or irrelevant.
- If a habit has excellent completion rate, suggest 'keep_same' and praise them.
- Ensure 'adjustmentType' is EXACTLY one of: "reduce_difficulty", "change_time", "clarify_plan", "keep_same", "pause_habit".

Rules for AI Summary:
- Keep the summary engaging, direct, and matching the requested coaching tone.
- Do NOT diagnose any medical/mental health conditions (keep it focused strictly on productivity/accountability).
- If there are severe blocker notes indicating crisis or self-harm, insert standard supportive/safety routing advice and do not prescribe habits.

Return ONLY the raw JSON. Do not include any other markdown wrapper other than standard json string formatting if necessary.`
}

// JSONDelimiter separates the streamed summary text from the structured JSON.
const JSONDelimiter = "|||JSON|||"

// BuildWeeklyReviewStreamSystemPrompt is a streaming-specific prompt. It tells the
// model to emit the human-readable summary first, then the JSON delimiter, then a
// small JSON object containing only the structured fields.
func BuildWeeklyReviewStreamSystemPrompt(style string, preferredTone string, difficultyPreference string) string {
	var tone string
	switch style {
	case "gentle":
		tone = `You are a gentle, nurturing accountability coach. You validate struggles, celebrate small progress, avoid intense pressure, and provide a warm, encouraging environment to rebuild momentum.`
	case "strict":
		tone = `You are a strict, direct, no-nonsense accountability coach. You cut through excuses, highlight gaps in execution, and challenge the user to step up, while remaining constructive, professional, and respectful.`
	default: // "balanced"
		tone = `You are a balanced accountability coach. You offer constructive, objective feedback, celebrate clear wins, gently address challenges, and give practical, actionable suggestions.`
	}

	switch preferredTone {
	case "supportive":
		tone += ` Your communication style is warm, empathetic, and encouraging. You focus on building confidence and celebrating progress.`
	case "direct":
		tone += ` Your communication style is straightforward, concise, and action-oriented. You focus on clear steps and measurable outcomes.`
	case "warm":
		tone += ` Your communication style is friendly, approachable, and conversational. You build rapport through relatable language.`
	case "practical":
		tone += ` Your communication style is pragmatic, solution-focused, and implementation-oriented. You emphasize actionable tactics over theory.`
	case "challenging":
		tone += ` Your communication style is motivating and pushes for growth. You challenge the user to exceed their current capabilities while remaining supportive.`
	}

	var difficultyGuidance string
	switch difficultyPreference {
	case "easy":
		difficultyGuidance = ` Suggest smaller, easily achievable steps. Focus on building momentum with minimal friction.`
	case "adaptive":
		difficultyGuidance = ` Adjust suggestions based on their current performance - easier when struggling, more challenging when excelling.`
	case "ambitious":
		difficultyGuidance = ` Suggest challenging but achievable goals that push for growth. Focus on rapid improvement and skill development.`
	default:
		difficultyGuidance = ` Provide balanced suggestions that match their current capability level.`
	}

	return tone + difficultyGuidance + `

Your task is to analyze the user's weekly habit performance and generate a coaching review.

IMPORTANT - OUTPUT FORMAT FOR STREAMING:
1. First, write the coaching summary as plain readable text (3-4 sentences). No JSON, no quotes, no markdown code fences.
2. Then, on a new line by itself, write exactly: ` + JSONDelimiter + `
3. Then, on a new line, write a JSON object with ONLY these two fields:
   {"suggestedAdjustments": [...], "nextWeekPlan": {...}}
   Do NOT include "aiSummary" in the JSON object.

Do not include any text after the JSON.

Example of correct output:
This week you built strong momentum, finishing with an 85% completion rate. Your best day was Wednesday, and the only real friction was a scheduling conflict on Monday. Keep the current routine and protect your streak next week.
` + JSONDelimiter + `
{"suggestedAdjustments": [{"habitId": "habit-1", "habitName": "Morning meditation", "adjustmentType": "keep_same", "reason": "You maintained strong consistency.", "suggestion": "Continue your current 10-minute practice each morning."}], "nextWeekPlan": {"focus": "Protect the streak", "commitments": ["Meditate 10 minutes each morning", "Log the session before breakfast"], "risks": ["A busy Monday morning could break the streak"], "recoveryActions": ["If you miss the morning, do a 2-minute session before bed"]}}

Rules for Suggested Adjustments:
- If a habit has low completion rate (e.g., < 50%), suggest adjustments like:
  - 'reduce_difficulty': Scale back volume or complexity.
  - 'change_time': Align habit to high energy periods or stable times.
  - 'clarify_plan': Redefine exact triggers or context.
  - 'pause_habit': Temporary pause if it's currently overwhelming or irrelevant.
- If a habit has excellent completion rate, suggest 'keep_same' and praise them.
- Ensure 'adjustmentType' is EXACTLY one of: "reduce_difficulty", "change_time", "clarify_plan", "keep_same", "pause_habit".

Rules for Next Week Plan:
- focus: a central theme/priority for next week.
- commitments: 2-3 specific, realistic habit commitments.
- risks: 1-2 potential blockers or risks predicted from last week's data.
- recoveryActions: 1-2 actionable recovery plans if they slip up.

Rules for the Plain Text Summary:
- Keep the summary engaging, direct, and matching the requested coaching tone.
- Do NOT diagnose any medical/mental health conditions (keep it focused strictly on productivity/accountability).
- If there are severe blocker notes indicating crisis or self-harm, insert standard supportive/safety routing advice and do not prescribe habits.`
}

// BuildWeeklyReviewUserPrompt serializes user activity, habits, blockers, and stats for the LLM.
func BuildWeeklyReviewUserPrompt(in WeeklyReviewInput) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Accountability Style: %s\n", in.AccountabilityStyle)
	if len(in.Goals) > 0 {
		fmt.Fprintf(&b, "Active Goals: %s\n", strings.Join(in.Goals, ", "))
	} else {
		fmt.Fprintf(&b, "Active Goals: None set\n")
	}

	fmt.Fprintf(&b, "\n--- Weekly Statistics ---\n")
	fmt.Fprintf(&b, "Total Habits: %d\n", in.TotalHabits)
	fmt.Fprintf(&b, "Completion Rate: %.1f%%\n", in.CompletionRate)
	fmt.Fprintf(&b, "Completed Check-ins: %d\n", in.CompletedCheckIns)
	fmt.Fprintf(&b, "Missed Check-ins: %d\n", in.MissedCheckIns)
	fmt.Fprintf(&b, "Best Day: %s\n", orDefaultString(in.BestDay, "None"))
	fmt.Fprintf(&b, "Hardest Day: %s\n", orDefaultString(in.HardestDay, "None"))
	fmt.Fprintf(&b, "Top Blocker: %s\n", orDefaultString(in.TopBlocker, "None"))

	fmt.Fprintf(&b, "\n--- Habit Breakdown ---\n")
	for _, h := range in.HabitBreakdowns {
		fmt.Fprintf(&b, "- %s: %d/%d (%.1f%% complete)\n", h.HabitName, h.CompletedCount, h.CompletedCount+h.MissedCount, h.CompletionRate)
	}

	fmt.Fprintf(&b, "\n--- Mood Frequencies ---\n")
	if len(in.MoodStats) > 0 {
		for _, m := range in.MoodStats {
			fmt.Fprintf(&b, "- %s: %d times\n", m.Mood, m.Count)
		}
	} else {
		fmt.Fprintf(&b, "No mood data recorded.\n")
	}

	fmt.Fprintf(&b, "\n--- Energy Frequencies ---\n")
	if len(in.EnergyStats) > 0 {
		for _, e := range in.EnergyStats {
			fmt.Fprintf(&b, "- %s: %d times\n", e.Energy, e.Count)
		}
	} else {
		fmt.Fprintf(&b, "No energy data recorded.\n")
	}

	fmt.Fprintf(&b, "\n--- Blocker Details ---\n")
	if len(in.BlockerStats) > 0 {
		for _, blk := range in.BlockerStats {
			fmt.Fprintf(&b, "- %s: %d times\n", blk.Blocker, blk.Count)
		}
	} else {
		fmt.Fprintf(&b, "No blockers encountered. Excellent job!\n")
	}

	if len(in.DetectedPatterns) > 0 {
		fmt.Fprintf(&b, "\n--- Detected Patterns ---\n")
		for _, pattern := range in.DetectedPatterns {
			fmt.Fprintf(&b, "- %s\n", pattern)
		}
	}

	return b.String()
}

// GenerateDeterministicFallback constructs a rich, deterministic coach review in case the LLM fails.
func GenerateDeterministicFallback(in WeeklyReviewInput) WeeklyReviewStructuredOutput {
	var summary string
	var focus string
	var commitments []string
	var risks []string
	var recovery []string

	bestDayStr := orDefaultString(in.BestDay, "some days")
	hardestDayStr := orDefaultString(in.HardestDay, "other days")

	if in.CompletionRate >= 80 {
		habitNames := make([]string, 0, len(in.HabitBreakdowns))
		for _, h := range in.HabitBreakdowns {
			if h.HabitName != "" {
				habitNames = append(habitNames, h.HabitName)
			}
		}
		habitList := "your habits"
		if len(habitNames) == 1 {
			habitList = habitNames[0]
		} else if len(habitNames) > 1 {
			habitList = strings.Join(habitNames[:len(habitNames)-1], ", ") + " and " + habitNames[len(habitNames)-1]
		}
		summary = fmt.Sprintf("Excellent work this week! A %.1f%% completion rate shows real discipline across %s. Your best momentum peaked on %s, and you handled the friction on %s well. Let's build on this strong foundation and keep the streak alive next week.", in.CompletionRate, habitList, bestDayStr, hardestDayStr)
		focus = "Maintain Excellence & Prevent Burnout"
		commitments = []string{
			"Keep up the high-frequency habit check-ins.",
			"Celebrate this high-performing week to reinforce positive habit loops.",
		}
		risks = []string{"Over-commitment or creeping fatigue."}
		recovery = []string{"Schedule a light rest day or dial back intensity slightly if fatigue sets in."}
	} else if in.CompletionRate >= 50 {
		blocker := orDefaultString(in.TopBlocker, "general schedule conflicts")
		summary = fmt.Sprintf("A solid week of growth. You hit a %.1f%% completion rate, showing strong consistency especially on %s. %s was the biggest challenge. By adjusting around %s, we can unlock even higher consistency next week.", in.CompletionRate, bestDayStr, hardestDayStr, blocker)
		focus = "Optimize Friction & Solidify Routine"
		commitments = []string{
			"Target the specific day/time you typically miss.",
			"Reduce friction by planning habit materials ahead of time.",
		}
		risks = []string{"Letting mid-week blockers break consecutive streaks."}
		recovery = []string{"If a blocker occurs, perform a 'micro' version of the habit (e.g. 2 minutes instead of 20) to keep the neural pathway active."}
	} else {
		habitNames := make([]string, 0, len(in.HabitBreakdowns))
		for _, h := range in.HabitBreakdowns {
			if h.HabitName != "" {
				habitNames = append(habitNames, h.HabitName)
			}
		}
		habitList := "your habits"
		if len(habitNames) == 1 {
			habitList = habitNames[0]
		} else if len(habitNames) > 1 {
			habitList = strings.Join(habitNames[:len(habitNames)-1], ", ") + " and " + habitNames[len(habitNames)-1]
		}
		blocker := orDefaultString(in.TopBlocker, "the week's friction points")
		summary = fmt.Sprintf("This week brought friction around %s, landing at a %.1f%% completion rate. That's completely okay—every challenging week is valuable diagnostic data. You had bright spots on %s, but the most friction showed up on %s, driven mainly by %s. Next week, let's lower the pressure, simplify the rules, and lock in one or two easy wins to rebuild momentum.", habitList, in.CompletionRate, bestDayStr, hardestDayStr, blocker)
		focus = "Lower the Bar to Re-establish Momentum"
		commitments = []string{
			"Reduce habit scope or difficulty by 50% to guarantee completion.",
			"Check in daily, even if it's to mark a habit as missed, to maintain the tracking routine.",
		}
		risks = []string{"A feeling of overwhelm leading to complete avoidance."}
		recovery = []string{"If you feel resistance, tell yourself you will only do the habit for 1 single minute, and then allow yourself to stop."}
	}

	adjustments := make([]WeeklyReviewAdjustment, 0)
	for _, h := range in.HabitBreakdowns {
		var adj WeeklyReviewAdjustment
		adj.HabitID = h.HabitID
		adj.HabitName = h.HabitName

		if h.CompletionRate >= 80 {
			adj.AdjustmentType = "keep_same"
			adj.Reason = "You maintained exceptional momentum and consistency."
			adj.Suggestion = "Keep doing exactly what you are doing. The timing and friction levels are optimized."
		} else if h.CompletionRate >= 50 {
			adj.AdjustmentType = "clarify_plan"
			adj.Reason = "You have decent consistency but encounter occasional scheduling friction."
			adj.Suggestion = fmt.Sprintf("Define an explicit 'If-Then' implementation plan (e.g., 'If I am at my desk at 9 AM, Then I will do %s').", h.HabitName)
		} else {
			adj.AdjustmentType = "reduce_difficulty"
			adj.Reason = "This habit encountered significant resistance or scheduling blocks."
			adj.Suggestion = "Reduce the daily volume or scope of this habit by half. Make the target so small it is almost impossible to fail."
		}
		adjustments = append(adjustments, adj)
	}

	if len(adjustments) == 0 {
		adjustments = append(adjustments, WeeklyReviewAdjustment{
			HabitID:        "",
			HabitName:      "General Routine",
			Reason:         "No habits were active this week.",
			Suggestion:     "Create at least one small, atomic habit to kickstart your growth loop.",
			AdjustmentType: "clarify_plan",
		})
	}

	return WeeklyReviewStructuredOutput{
		AiSummary:            summary,
		SuggestedAdjustments: adjustments,
		NextWeekPlan: WeeklyReviewNextWeekPlan{
			Focus:           focus,
			Commitments:     commitments,
			Risks:           risks,
			RecoveryActions: recovery,
		},
	}
}

func orDefaultString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
