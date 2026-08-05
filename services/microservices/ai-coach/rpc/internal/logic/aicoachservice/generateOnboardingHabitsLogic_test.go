package aicoachservicelogic

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/aitest"
	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"
)

func TestParseOnboardingHabits_PlainJSON(t *testing.T) {
	habits, err := parseOnboardingHabits(`[
	  {"name":"Walk 15 min after lunch","description":"Right after you finish eating."},
	  {"name":"Read 10 pages","description":"Before bed each night."},
	  {"name":"Log one win","description":"Write it in your journal."}
	]`)
	require.NoError(t, err)
	require.Len(t, habits, 3)
	assert.Equal(t, "Walk 15 min after lunch", habits[0].Name)
}

func TestParseOnboardingHabits_MarkdownFenceAndSurroundingText(t *testing.T) {
	out := "Sure! Here are your habits:\n" +
		"```json\n" +
		`[{"name":"Meditate","description":"5 minutes each morning"},{"name":"Hydrate","description":"Drink water on waking"}]` + "\n" +
		"```\n" +
		"Hope that helps!"
	habits, err := parseOnboardingHabits(out)
	require.NoError(t, err)
	require.Len(t, habits, 2)
	assert.Equal(t, "Meditate", habits[0].Name)
}

func TestParseOnboardingHabits_TruncatesToThree(t *testing.T) {
	habits, err := parseOnboardingHabits(`[
	  {"name":"a","description":"x"},{"name":"b","description":"x"},
	  {"name":"c","description":"x"},{"name":"d","description":"x"}
	]`)
	require.NoError(t, err)
	assert.Len(t, habits, 3)
}

func TestParseOnboardingHabits_EmptyNamesSkipped(t *testing.T) {
	habits, err := parseOnboardingHabits(`[{"name":"","description":"x"},{"name":"real","description":"x"}]`)
	require.NoError(t, err)
	require.Len(t, habits, 1)
	assert.Equal(t, "real", habits[0].Name)
}

func TestParseOnboardingHabits_NoArray(t *testing.T) {
	_, err := parseOnboardingHabits("no json here at all")
	assert.Error(t, err)
}

func TestGenerateOnboardingHabits_CrisisBlockedUsesFallback(t *testing.T) {
	mc := aitest.NewMockClient()
	// Classifier flags the combined free-text as crisis.
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `{"category":"crisis","confidence":0.9,"reason":"distress"}`,
	}, ai.Usage{}, 0)
	// No ModelChat response recorded — the model must NOT be called.

	svcCtx := &svc.ServiceContext{AIClient: mc, Classifier: safety.NewLLMClassifier(mc)}
	logic := NewGenerateOnboardingHabitsLogic(context.Background(), svcCtx)

	resp, err := logic.GenerateOnboardingHabits(&aicoach.GenerateOnboardingHabitsRequest{
		UserId:       "user-1",
		GoalTitle:    "Get fit",
		DailyMinutes: 30,
		Motivation:   "I can't take this anymore",
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Habits)
	// Fallback habits mention the goal title; the model was never called.
	assert.Contains(t, resp.Habits[0].Name, "Get fit")
}

func TestGenerateOnboardingHabits_SafeGeneratesHabits(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `{"category":"safe","confidence":0.99,"reason":"ok"}`,
	}, ai.Usage{}, 0)
	mc.RecordResponse(ai.ModelChat, ai.Message{
		Role: ai.RoleAssistant,
		Content: `[{"name":"Walk 15 min after lunch","description":"Right after eating."},
{"name":"Read 10 pages","description":"Before bed."},
{"name":"Log one win","description":"In your journal."}]`,
	}, ai.Usage{}, 0)

	svcCtx := &svc.ServiceContext{AIClient: mc, Classifier: safety.NewLLMClassifier(mc)}
	logic := NewGenerateOnboardingHabitsLogic(context.Background(), svcCtx)

	resp, err := logic.GenerateOnboardingHabits(&aicoach.GenerateOnboardingHabitsRequest{
		UserId:              "user-1",
		GoalTitle:           "Read more",
		GoalCategory:        "learning",
		Motivation:          "Grow my knowledge",
		Blocker:             "Time",
		DailyMinutes:        30,
		AccountabilityStyle: "balanced",
	})
	require.NoError(t, err)
	require.Len(t, resp.Habits, 3)
	assert.Equal(t, "Walk 15 min after lunch", resp.Habits[0].Name)
}

// ============================================
// Additional edge cases
// ============================================

func TestGenerateOnboardingHabits_NilAIClientUsesFallback(t *testing.T) {
	// No AIClient and no Classifier → deterministic fallback habits.
	svcCtx := &svc.ServiceContext{AIClient: nil, Classifier: nil}
	logic := NewGenerateOnboardingHabitsLogic(context.Background(), svcCtx)

	resp, err := logic.GenerateOnboardingHabits(&aicoach.GenerateOnboardingHabitsRequest{
		UserId:       "user-1",
		GoalTitle:    "Get fit",
		DailyMinutes: 30,
	})
	require.NoError(t, err)
	require.Len(t, resp.Habits, 3)
	// Fallback first habit references the goal title
	assert.Contains(t, resp.Habits[0].Name, "Get fit")
	assert.Contains(t, resp.Habits[0].Name, "10 minutes") // 30/3 = 10
}

func TestGenerateOnboardingHabits_AIGenerateErrorUsesFallback(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `{"category":"safe","confidence":0.99,"reason":"ok"}`,
	}, ai.Usage{}, 0)
	// No ModelChat response recorded → Generate returns an error for that profile.
	// But we want a concrete error; set a global error instead to simulate AI failure.
	mc.SetError(errors.New("AI provider unavailable"))

	svcCtx := &svc.ServiceContext{AIClient: mc, Classifier: safety.NewLLMClassifier(mc)}
	logic := NewGenerateOnboardingHabitsLogic(context.Background(), svcCtx)

	resp, err := logic.GenerateOnboardingHabits(&aicoach.GenerateOnboardingHabitsRequest{
		UserId:       "user-1",
		GoalTitle:    "Read more",
		DailyMinutes: 45,
	})
	// Onboarding never blocks the user — fallback habits, no error.
	require.NoError(t, err)
	require.NotEmpty(t, resp.Habits)
	assert.Contains(t, resp.Habits[0].Name, "Read more")
}

func TestGenerateOnboardingHabits_SelfHarmBlockedUsesFallback(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `{"category":"self_harm","confidence":0.85,"reason":"self-harm ideation"}`,
	}, ai.Usage{}, 0)
	// No ModelChat response — the model must NOT be called for self-harm input.

	svcCtx := &svc.ServiceContext{AIClient: mc, Classifier: safety.NewLLMClassifier(mc)}
	logic := NewGenerateOnboardingHabitsLogic(context.Background(), svcCtx)

	resp, err := logic.GenerateOnboardingHabits(&aicoach.GenerateOnboardingHabitsRequest{
		UserId:       "user-1",
		GoalTitle:    "Get fit",
		DailyMinutes: 30,
		Motivation:   "I want to hurt myself",
	})
	require.NoError(t, err)
	require.NotEmpty(t, resp.Habits)
	// Fallback habits; the model was never called with the harmful input.
	assert.Contains(t, resp.Habits[0].Name, "Get fit")
}

func TestGenerateOnboardingHabits_ClassifierErrorProceedsToGeneration(t *testing.T) {
	mc := aitest.NewMockClient()
	// Force the classifier call to fail by setting a global error, then clear it
	// before the chat call. We use two separate mock clients to isolate failures:
	// classifier client errors, chat client succeeds.
	classifierClient := aitest.NewMockClient()
	classifierClient.SetError(errors.New("classifier down"))

	chatClient := aitest.NewMockClient()
	chatClient.RecordResponse(ai.ModelChat, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `[{"name":"Walk","description":"daily"},{"name":"Read","description":"nightly"},{"name":"Log","description":"journal"}]`,
	}, ai.Usage{}, 0)

	// Use the erroring client only for the classifier; the logic uses the same
	// svcCtx.AIClient for both, so we need a single client that errors on
	// classifier then succeeds on chat. Simplest: a client that has no recorded
	// classifier response (returns error) but has a recorded chat response.
	mc.RecordResponse(ai.ModelChat, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `[{"name":"Walk","description":"daily"},{"name":"Read","description":"nightly"},{"name":"Log","description":"journal"}]`,
	}, ai.Usage{}, 0)
	// No ModelClassifier response recorded → classifier call errors.
	// The logic logs the error and proceeds to generation.

	svcCtx := &svc.ServiceContext{AIClient: mc, Classifier: safety.NewLLMClassifier(mc)}
	logic := NewGenerateOnboardingHabitsLogic(context.Background(), svcCtx)

	resp, err := logic.GenerateOnboardingHabits(&aicoach.GenerateOnboardingHabitsRequest{
		UserId:       "user-1",
		GoalTitle:    "Read more",
		DailyMinutes: 30,
		Motivation:   "Grow",
	})
	// Classifier error is logged, not returned — generation proceeds.
	require.NoError(t, err)
	require.Len(t, resp.Habits, 3)
	assert.Equal(t, "Walk", resp.Habits[0].Name)
}

func TestGenerateOnboardingHabits_EmptyFreeTextStillGeneratesHabits(t *testing.T) {
	mc := aitest.NewMockClient()
	// Record a classifier response (safe) and a chat response.
	// Even with empty free-text, strings.Join produces "\n\n" (not ""), so the
	// classifier is still called — this verifies the flow handles that gracefully.
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `{"category":"safe","confidence":0.99,"reason":"ok"}`,
	}, ai.Usage{}, 0)
	mc.RecordResponse(ai.ModelChat, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `[{"name":"Habit A","description":"d"},{"name":"Habit B","description":"d"},{"name":"Habit C","description":"d"}]`,
	}, ai.Usage{}, 0)

	svcCtx := &svc.ServiceContext{AIClient: mc, Classifier: safety.NewLLMClassifier(mc)}
	logic := NewGenerateOnboardingHabitsLogic(context.Background(), svcCtx)

	// All free-text fields empty — the flow still works end-to-end.
	resp, err := logic.GenerateOnboardingHabits(&aicoach.GenerateOnboardingHabitsRequest{
		UserId:       "user-1",
		GoalTitle:    "",
		Motivation:   "",
		Blocker:      "",
		DailyMinutes: 30,
	})
	require.NoError(t, err)
	require.Len(t, resp.Habits, 3)
	assert.Equal(t, "Habit A", resp.Habits[0].Name)
}

func TestGenerateOnboardingHabits_UnparseableAIResponseUsesFallback(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `{"category":"safe","confidence":0.99,"reason":"ok"}`,
	}, ai.Usage{}, 0)
	mc.RecordResponse(ai.ModelChat, ai.Message{
		Role:    ai.RoleAssistant,
		Content: "Sorry, I can't help with that.", // no JSON array
	}, ai.Usage{}, 0)

	svcCtx := &svc.ServiceContext{AIClient: mc, Classifier: safety.NewLLMClassifier(mc)}
	logic := NewGenerateOnboardingHabitsLogic(context.Background(), svcCtx)

	resp, err := logic.GenerateOnboardingHabits(&aicoach.GenerateOnboardingHabitsRequest{
		UserId:       "user-1",
		GoalTitle:    "Read more",
		DailyMinutes: 30,
	})
	// Parse failure → fallback habits, no error returned.
	require.NoError(t, err)
	require.NotEmpty(t, resp.Habits)
	assert.Contains(t, resp.Habits[0].Name, "Read more")
}

// ============================================
// Fallback habit math
// ============================================

func TestOnboardingFallbackHabits_DailyMinutesDividedByThree(t *testing.T) {
	habits := onboardingFallbackHabits("My goal", 30)
	require.Len(t, habits, 3)
	// 30 / 3 = 10 minutes per session
	assert.Contains(t, habits[0].Name, "My goal")
	assert.Contains(t, habits[0].Name, "10 minutes")
}

func TestOnboardingFallbackHabits_ZeroDailyMinutesDefaultsToThree(t *testing.T) {
	habits := onboardingFallbackHabits("My goal", 0)
	require.Len(t, habits, 3)
	// 0 daily minutes → per defaults to 3
	assert.Contains(t, habits[0].Name, "3 minutes")
}

func TestOnboardingFallbackHabits_LessThanThreeMinutesClampsToOne(t *testing.T) {
	habits := onboardingFallbackHabits("My goal", 2)
	require.Len(t, habits, 3)
	// 2 / 3 = 0 → clamped to 1
	assert.Contains(t, habits[0].Name, "1 minutes")
}

func TestOnboardingFallbackHabits_AlwaysReturnsThreeHabits(t *testing.T) {
	for _, mins := range []int32{1, 15, 30, 45, 60, 90, 600} {
		habits := onboardingFallbackHabits("Goal", mins)
		assert.Len(t, habits, 3, "expected 3 fallback habits for dailyMinutes=%d", mins)
		// Second and third habits are static
		assert.Equal(t, "Review your plan for tomorrow", habits[1].Name)
		assert.Equal(t, "Track your progress", habits[2].Name)
	}
}

// ============================================
// parseOnboardingHabits additional edge cases
// ============================================

func TestParseOnboardingHabits_NestedBracketsInText(t *testing.T) {
	// Text contains brackets but the JSON array is still found.
	out := "Here are habits: [{\"name\":\"A\",\"description\":\"d\"},{\"name\":\"B\",\"description\":\"d\"},{\"name\":\"C\",\"description\":\"d\"}] done"
	habits, err := parseOnboardingHabits(out)
	require.NoError(t, err)
	assert.Len(t, habits, 3)
	assert.Equal(t, "A", habits[0].Name)
}

func TestParseOnboardingHabits_TrimsWhitespaceInNames(t *testing.T) {
	habits, err := parseOnboardingHabits(`[{"name":"  spaced  ","description":"  desc  "}]`)
	require.NoError(t, err)
	require.Len(t, habits, 1)
	assert.Equal(t, "spaced", habits[0].Name)
	assert.Equal(t, "desc", habits[0].Description)
}

func TestParseOnboardingHabits_AllEmptyNamesReturnsError(t *testing.T) {
	_, err := parseOnboardingHabits(`[{"name":"","description":"x"},{"name":"   ","description":"x"}]`)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no valid habits")
}

func TestParseOnboardingHabits_ReversedBracketsReturnsError(t *testing.T) {
	_, err := parseOnboardingHabits("]not json[")
	assert.Error(t, err)
}

func TestParseOnboardingHabits_CodeFenceWithoutJsonPrefix(t *testing.T) {
	out := "```\n[{\"name\":\"A\",\"description\":\"d\"},{\"name\":\"B\",\"description\":\"d\"},{\"name\":\"C\",\"description\":\"d\"}]\n```"
	habits, err := parseOnboardingHabits(out)
	require.NoError(t, err)
	assert.Len(t, habits, 3)
}

func TestParseOnboardingHabits_InvalidJSONReturnsError(t *testing.T) {
	_, err := parseOnboardingHabits("[{invalid json}]")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unmarshal")
}

func TestParseOnboardingHabits_PreservesOrderUpToThree(t *testing.T) {
	habits, err := parseOnboardingHabits(`[
	  {"name":"first","description":"d"},
	  {"name":"second","description":"d"},
	  {"name":"third","description":"d"},
	  {"name":"fourth","description":"d"}
	]`)
	require.NoError(t, err)
	require.Len(t, habits, 3)
	assert.Equal(t, "first", habits[0].Name)
	assert.Equal(t, "second", habits[1].Name)
	assert.Equal(t, "third", habits[2].Name)
}

func TestParseOnboardingHabits_EmptyStringReturnsError(t *testing.T) {
	_, err := parseOnboardingHabits("")
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "no JSON array") || strings.Contains(err.Error(), "no valid"))
}
