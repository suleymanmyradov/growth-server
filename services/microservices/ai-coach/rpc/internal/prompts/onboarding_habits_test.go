package prompts

import (
	"strings"
	"testing"
)

func TestBuildOnboardingHabitsSystemPrompt_IncludesAllFields(t *testing.T) {
	in := OnboardingHabitsInput{
		GoalTitle:           "Run a 5k",
		GoalCategory:        "fitness",
		Motivation:          "Improve my health and energy",
		Blocker:             "Lack of time in the morning",
		DailyMinutes:        30,
		AccountabilityStyle: "strict",
	}
	prompt := BuildOnboardingHabitsSystemPrompt(in)

	cases := []struct {
		label string
		want  string
	}{
		{"goal title", "Run a 5k"},
		{"goal category", "fitness"},
		{"motivation", "Improve my health and energy"},
		{"blocker", "Lack of time in the morning"},
		{"daily minutes", "30 minutes"},
		{"accountability style", "strict"},
	}
	for _, c := range cases {
		if !strings.Contains(prompt, c.want) {
			t.Errorf("prompt missing %s: expected to contain %q\nprompt:\n%s", c.label, c.want, prompt)
		}
	}
}

func TestBuildOnboardingHabitsSystemPrompt_ContainsJSONShape(t *testing.T) {
	prompt := BuildOnboardingHabitsSystemPrompt(OnboardingHabitsInput{
		GoalTitle:    "Test",
		DailyMinutes: 15,
	})
	// The JSON shape must be present so the model knows the exact format.
	if !strings.Contains(prompt, `"name": "<habit name>"`) {
		t.Errorf("prompt missing JSON shape with name field")
	}
	if !strings.Contains(prompt, `"description": "<one sentence describing when/how to do it>"`) {
		t.Errorf("prompt missing JSON shape with description field")
	}
}

func TestBuildOnboardingHabitsSystemPrompt_DailyMinutesInRules(t *testing.T) {
	// The "fit within X minutes total combined" rule uses DailyMinutes.
	prompt := BuildOnboardingHabitsSystemPrompt(OnboardingHabitsInput{
		GoalTitle:    "Test",
		DailyMinutes: 45,
	})
	if !strings.Contains(prompt, "fit within 45 minutes total combined") {
		t.Errorf("prompt missing daily minutes in rules: expected 'fit within 45 minutes total combined'")
	}
}

func TestBuildOnboardingHabitsSystemPrompt_EmptyOptionalFields(t *testing.T) {
	// Optional fields empty — prompt still builds without error and includes defaults.
	prompt := BuildOnboardingHabitsSystemPrompt(OnboardingHabitsInput{
		GoalTitle:    "Read more",
		DailyMinutes: 30,
		// GoalCategory, Motivation, Blocker, AccountabilityStyle all empty
	})
	if !strings.Contains(prompt, "Read more") {
		t.Errorf("prompt missing goal title")
	}
	if !strings.Contains(prompt, "30 minutes") {
		t.Errorf("prompt missing daily minutes")
	}
}

func TestBuildOnboardingHabitsSystemPrompt_ReturnsNonEmpty(t *testing.T) {
	prompt := BuildOnboardingHabitsSystemPrompt(OnboardingHabitsInput{
		GoalTitle:    "Test",
		DailyMinutes: 30,
	})
	if len(strings.TrimSpace(prompt)) == 0 {
		t.Fatal("prompt should not be empty")
	}
}

func TestBuildOnboardingHabitsSystemPrompt_CoachRoleStated(t *testing.T) {
	prompt := BuildOnboardingHabitsSystemPrompt(OnboardingHabitsInput{
		GoalTitle:    "Test",
		DailyMinutes: 30,
	})
	if !strings.Contains(prompt, "AI accountability coach") {
		t.Errorf("prompt should state the AI accountability coach role")
	}
	if !strings.Contains(prompt, "exactly 3") {
		t.Errorf("prompt should specify exactly 3 habits")
	}
}
