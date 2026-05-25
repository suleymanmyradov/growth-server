package prompts

import (
	"fmt"
	"strings"
)

// PersonalizedCoachingInput holds the data needed for personalized coaching prompts
type PersonalizedCoachingInput struct {
	UserMessage           string
	AccountabilityStyle   string
	PreferredTone         string
	DifficultyPreference  string
	ActiveGoals           []string
	ActiveHabits          []string
	RecentCheckInsSummary string
	CommonBlockers        []string
	PatternInsights       map[string]string
}

// BuildPersonalizedCoachingSystemPrompt creates a system prompt for personalized AI coaching
func BuildPersonalizedCoachingSystemPrompt(profile PersonalizedCoachingInput) string {
	var tone string

	// Build tone based on user preferences
	switch profile.PreferredTone {
	case "supportive":
		tone = "You are a warm, encouraging accountability coach. You validate feelings, celebrate wins, and provide gentle guidance. You focus on building confidence and motivation."
	case "direct":
		tone = "You are a direct, straightforward accountability coach. You communicate clearly and concisely, avoiding fluff. You focus on practical advice and actionable steps."
	case "warm":
		tone = "You are a friendly, approachable accountability coach. You create a safe space for honest conversation. You use empathetic language while maintaining focus on goals."
	case "practical":
		tone = "You are a pragmatic, results-oriented accountability coach. You focus on concrete strategies and realistic solutions. You avoid abstract advice in favor of specific tactics."
	case "challenging":
		tone = "You are a challenging, growth-oriented accountability coach. You push users to stretch beyond their comfort zone while remaining supportive. You focus on potential and possibility."
	default:
		tone = "You are a balanced accountability coach. You offer constructive feedback, celebrate wins, and provide practical guidance."
	}

	// Add accountability style modifiers
	switch profile.AccountabilityStyle {
	case "gentle":
		tone += " You prioritize emotional support and understanding. You avoid pressure and focus on gradual progress."
	case "strict":
		tone += " You maintain high standards and direct accountability. You focus on commitment and results while remaining respectful."
	case "balanced":
		tone += " You balance support with appropriate accountability. You recognize both effort and outcomes."
	}

	// Add difficulty preference guidance
	switch profile.DifficultyPreference {
	case "easy":
		tone += " You suggest manageable, low-friction steps that build momentum gradually."
	case "adaptive":
		tone += " You adjust your suggestions based on the user's current capacity and context."
	case "ambitious":
		tone += " You encourage challenging goals and significant growth while ensuring they remain achievable."
	}

	return tone + `

Rules:
- Keep responses concise (3-5 sentences for most interactions)
- Be specific to the user's goals, habits, and current situation
- Reference their actual habits and goals when relevant
- Acknowledge their patterns and common blockers
- Never diagnose medical or mental health conditions
- If user expresses crisis or self-harm thoughts, provide supportive resources
- Focus on behavioral change and accountability strategies
- Suggest concrete, actionable next steps
- Celebrate progress and acknowledge effort
- Address setbacks constructively without judgment
- Maintain consistency with the established coaching tone`
}

// BuildPersonalizedCoachingUserPrompt creates a user prompt with personalization context
func BuildPersonalizedCoachingUserPrompt(profile PersonalizedCoachingInput) string {
	var b strings.Builder

	fmt.Fprintf(&b, "User's Message: %s\n\n", profile.UserMessage)

	fmt.Fprintf(&b, "--- Coaching Profile ---\n")
	fmt.Fprintf(&b, "Accountability Style: %s\n", profile.AccountabilityStyle)
	fmt.Fprintf(&b, "Preferred Tone: %s\n", profile.PreferredTone)
	fmt.Fprintf(&b, "Difficulty Preference: %s\n", profile.DifficultyPreference)

	if len(profile.ActiveGoals) > 0 {
		fmt.Fprintf(&b, "\n--- Active Goals ---\n")
		for i, goal := range profile.ActiveGoals {
			fmt.Fprintf(&b, "%d. %s\n", i+1, goal)
		}
	}

	if len(profile.ActiveHabits) > 0 {
		fmt.Fprintf(&b, "\n--- Active Habits ---\n")
		for i, habit := range profile.ActiveHabits {
			fmt.Fprintf(&b, "%d. %s\n", i+1, habit)
		}
	}

	if profile.RecentCheckInsSummary != "" {
		fmt.Fprintf(&b, "\n--- Recent Check-in Summary ---\n")
		fmt.Fprintf(&b, "%s\n", profile.RecentCheckInsSummary)
	}

	if len(profile.CommonBlockers) > 0 {
		fmt.Fprintf(&b, "\n--- Common Blockers ---\n")
		for i, blocker := range profile.CommonBlockers {
			fmt.Fprintf(&b, "%d. %s\n", i+1, blocker)
		}
	}

	if len(profile.PatternInsights) > 0 {
		fmt.Fprintf(&b, "\n--- Pattern Insights ---\n")
		// Sort keys for deterministic prompt generation
		keys := make([]string, 0, len(profile.PatternInsights))
		for key := range profile.PatternInsights {
			keys = append(keys, key)
		}
		for i := 0; i < len(keys); i++ {
			for j := i + 1; j < len(keys); j++ {
				if keys[i] > keys[j] {
					keys[i], keys[j] = keys[j], keys[i]
				}
			}
		}
		for _, key := range keys {
			fmt.Fprintf(&b, "%s: %s\n", key, profile.PatternInsights[key])
		}
	}

	return b.String()
}

// BuildContextSummary creates a concise summary of recent check-ins for the AI
func BuildContextSummary(checkInCount int, completionRate float64, topBlocker string, recentTrend string) string {
	var summary strings.Builder

	fmt.Fprintf(&summary, "Recent activity: %d check-ins with %.1f%% completion rate. ", checkInCount, completionRate)

	if topBlocker != "" {
		fmt.Fprintf(&summary, "Most common blocker: %s. ", topBlocker)
	}

	if recentTrend != "" {
		fmt.Fprintf(&summary, "Trend: %s. ", recentTrend)
	}

	return summary.String()
}
