package prompts

import (
	"fmt"
	"strings"
)

// CheckInFeedbackInput holds the data needed to render a check-in feedback prompt.
type CheckInFeedbackInput struct {
	HabitName            string
	Status               string // "completed" | "missed"
	Mood                 string
	Energy               string
	Blocker              string
	Note                 string
	AccountabilityStyle  string // "gentle" | "balanced" | "strict"
	PreferredTone        string // "supportive" | "direct" | "warm" | "practical" | "challenging"
	DifficultyPreference string // "easy" | "adaptive" | "ambitious"
	CommonBlockers       []string
	Streak               int32
	RecentPattern        string // e.g. "completed 5 of last 7 days"
}

// BuildSystemPrompt returns the system prompt for the check-in feedback coach.
func BuildSystemPrompt(style string, preferredTone string, difficultyPreference string) string {
	var tone string
	switch style {
	case "gentle":
		tone = `You are a gentle, warm accountability coach. You speak softly and encouragingly.
	You focus on emotional support and celebrate effort, not just results.
	You never use harsh language or pressure. You validate the user's feelings first.`
	case "strict":
		tone = `You are a direct, no-nonsense accountability coach. You are honest and challenging,
	but always respectful and constructive. You cut through excuses without being cruel.
	You focus on action and accountability. You do not sugarcoat.`
	default: // "balanced"
		tone = `You are a balanced accountability coach. You are supportive but honest.
	You celebrate wins and address misses with constructive feedback.
	You keep advice practical and concise. You avoid being either too soft or too harsh.`
	}

	// Apply preferred tone customization
	switch preferredTone {
	case "supportive":
		tone += ` Your communication style is warm, empathetic, and encouraging. You focus on building confidence.`
	case "direct":
		tone += ` Your communication style is straightforward, concise, and action-oriented. You focus on clear steps.`
	case "warm":
		tone += ` Your communication style is friendly, approachable, and conversational. You build rapport.`
	case "practical":
		tone += ` Your communication style is pragmatic, solution-focused, and implementation-oriented.`
	case "challenging":
		tone += ` Your communication style is motivating and pushes for growth. You challenge the user.`
	}

	// Apply difficulty preference
	var difficultyGuidance string
	switch difficultyPreference {
	case "easy":
		difficultyGuidance = ` Suggest smaller, easily achievable steps. Focus on building momentum.`
	case "adaptive":
		difficultyGuidance = ` Adjust suggestions based on their current performance.`
	case "ambitious":
		difficultyGuidance = ` Suggest challenging but achievable goals that push for growth.`
	default:
		difficultyGuidance = ` Provide balanced suggestions that match their current capability.`
	}

	return tone + difficultyGuidance + `

Rules:
- Respond in 2-3 sentences only.
- Be specific to the habit and situation described.
- Never be judgmental, shaming, or toxic.
- If completed: acknowledge the win, reinforce the streak, suggest keeping momentum.
- If missed: understand the blocker, suggest a small adjustment, protect tomorrow.
- Suggest one concrete next step or mindset shift.`
}

// BuildUserPrompt returns the user prompt with the check-in context.
func BuildUserPrompt(in CheckInFeedbackInput) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Habit: %s\n", in.HabitName)
	fmt.Fprintf(&b, "Status: %s\n", in.Status)

	if in.Mood != "" {
		fmt.Fprintf(&b, "Mood: %s\n", in.Mood)
	}
	if in.Energy != "" {
		fmt.Fprintf(&b, "Energy: %s\n", in.Energy)
	}
	if in.Blocker != "" {
		fmt.Fprintf(&b, "Blocker: %s\n", in.Blocker)
	}
	if in.Note != "" {
		fmt.Fprintf(&b, "Note: %s\n", in.Note)
	}

	fmt.Fprintf(&b, "Current streak: %d days\n", in.Streak)
	if in.RecentPattern != "" {
		fmt.Fprintf(&b, "Recent pattern: %s\n", in.RecentPattern)
	}

	return b.String()
}
