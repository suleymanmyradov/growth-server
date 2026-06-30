package prompts

import "fmt"

// OnboardingHabitsInput holds the structured onboarding data used to generate
// habit suggestions. All fields are server-controlled — the client never
// supplies a prompt, only this structured data.
type OnboardingHabitsInput struct {
	GoalTitle          string
	GoalCategory       string
	Motivation         string
	Blocker            string
	DailyMinutes       int32
	AccountabilityStyle string
}

// onboardingHabitsJSONShape is the exact JSON array shape the model must
// return. Kept here as the single source of truth for both the prompt and the
// parser.
const onboardingHabitsJSONShape = `[
  {"name": "<habit name>", "description": "<one sentence describing when/how to do it>"},
  {"name": "<habit name>", "description": "<one sentence describing when/how to do it>"},
  {"name": "<habit name>", "description": "<one sentence describing when/how to do it>"}
]`

// BuildOnboardingHabitsSystemPrompt is the server-owned system prompt for
// onboarding habit generation. It is never sent by the client.
func BuildOnboardingHabitsSystemPrompt(in OnboardingHabitsInput) string {
	return fmt.Sprintf(`You are an AI accountability coach. Generate exactly 3 specific, small, actionable daily habits for a user.

User context:
- Goal: %s (category: %s)
- Motivation: %s
- Main blocker: %s
- Daily time available: %d minutes
- Accountability style: %s

Rules:
- Each habit must fit within %d minutes total combined
- Habits must be small enough to do even on low-motivation days
- Be specific (not "exercise more" but "walk for 15 minutes after lunch")
- Return exactly this JSON format, no extra text, no markdown fences:
%s`,
		in.GoalTitle, in.GoalCategory, in.Motivation, in.Blocker,
		in.DailyMinutes, in.AccountabilityStyle,
		in.DailyMinutes,
		onboardingHabitsJSONShape,
	)
}
