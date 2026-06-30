package aicoachservicelogic

import (
	"context"
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
