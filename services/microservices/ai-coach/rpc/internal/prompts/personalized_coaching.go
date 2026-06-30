package prompts

import "time"

// Note: the priority-tiered context assembler (AssembleContextWithBreakdown)
// lives in context_assembler.go. BuildContextSummary (the shared check-in
// digest helper) lives in pkg/ai/prompts.

// CrisisResponse is the deterministic, never-model-generated response streamed
// to the user when the safety classifier detects a crisis or self-harm signal.
// It is a single source of truth so the model can never hallucinate or omit
// safety resources. Region-aware helplines can be derived from the user
// profile location later; for now this covers the most common regions.
const CrisisResponse = `It sounds like you're going through something really painful right now, and I'm glad you reached out.
I'm an accountability coach, not a crisis counselor, so I want to make sure you get the right support.

If you're in immediate danger, please call your local emergency number now.
• US: call or text 988 (Suicide & Crisis Lifeline)
• UK & ROI: call 116 123 (Samaritans)
• Elsewhere: https://findahelpline.com

You deserve support from someone trained to help. Would you like to keep talking about your goals when you're ready?`

// MemorySnippet is one retrieved long-term memory hit from the private
// user_memory index. It is treated as untrusted user data: the assembler
// wraps every snippet via prompts.WrapUserContent and SanitizeAndTruncate
// before it reaches the model.
type MemorySnippet struct {
	EntityType string    // check_in | conversation_message | weekly_review
	Content    string    // note / message / ai_summary
	CreatedAt  time.Time // when the source row was created
	HabitName  string    // check_in only — light metadata for attribution
	Role       string    // conversation_message only — user | assistant
}

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
	UserFullName          string
	UserBio               string
	UserLocation          string
	UserInterests         []string
	// RelevantMemories are cross-conversation, long-term memory snippets
	// retrieved from user_memory. They are injected at the LOWEST priority
	// tier of the context budget so they are the first section dropped when
	// the budget is tight, and never blow past the Stage-1 token budget.
	RelevantMemories []MemorySnippet
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
- Match your response to what the user actually said. If they greet you, make small talk, or ask a general question, respond naturally and conversationally — do NOT force coaching content, action steps, or references to their goals/habits onto a casual message.
- The coaching profile, goals, habits, check-in summary, blockers, and pattern insights are BACKGROUND context, not a mandate to bring them up. Only reference them when the user's message is actually about their goals, habits, progress, setbacks, or asks for coaching/help. Never assume the user is struggling, resuming after a pause, or asking for accountability just because this context is present.
- You may address the user by their first name when it feels natural, but don't force it into every response. Their bio, location, and interests are background context — reference them only when genuinely relevant to the conversation.
- When the user does ask for coaching, be specific to their goals, habits, and current situation; reference their actual habits and goals; acknowledge their patterns and common blockers; and suggest concrete, actionable next steps.
- Keep responses concise (3-5 sentences for most interactions, shorter for simple greetings or acknowledgements).
- Celebrate progress and acknowledge effort; address setbacks constructively without judgment.
- Never diagnose medical or mental health conditions.
- If the user expresses crisis or self-harm thoughts, provide supportive resources.
- Maintain consistency with the established coaching tone.`
}
