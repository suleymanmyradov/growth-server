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

func TestCheckInFreeText_JoinsAllUserFields(t *testing.T) {
	got := checkInFreeText(&aicoach.CheckInFeedbackRequest{
		HabitName:      "  Run 5k  ",
		Status:         "missed",
		Mood:           "low",
		Energy:         "drained",
		Blocker:        "injury",
		Note:           "feeling hopeless",
		RecentPattern:  "3 missed days",
		CommonBlockers: []string{"travel", "  ", ""},
	})
	want := "Run 5k\ninjury\nfeeling hopeless\n3 missed days\nmissed\nlow\ndrained\ntravel"
	assert.Equal(t, want, got)
}

func TestCheckInFreeText_EmptyWhenAllBlank(t *testing.T) {
	assert.Empty(t, checkInFreeText(&aicoach.CheckInFeedbackRequest{}))
	assert.Empty(t, checkInFreeText(&aicoach.CheckInFeedbackRequest{Note: "   "}))
}

// A1: blocker/note content must reach the classifier before the model.
func TestGenerateCheckInFeedback_ClassifiesFreeText(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `{"category":"safe","confidence":0.99,"reason":"ok"}`,
	}, ai.Usage{}, 0)
	mc.RecordResponse(ai.ModelCheap, ai.Message{
		Role:    ai.RoleAssistant,
		Content: "Great work!",
	}, ai.Usage{}, 0)

	svcCtx := &svc.ServiceContext{AIClient: mc, Classifier: safety.NewLLMClassifier(mc)}
	logic := NewGenerateCheckInFeedbackLogic(context.Background(), svcCtx)

	resp, err := logic.GenerateCheckInFeedback(&aicoach.CheckInFeedbackRequest{
		UserId:    "user-1",
		HabitName: "Meditate",
		Blocker:   "felt anxious",
		Note:      "skipped yesterday",
	})
	require.NoError(t, err)
	assert.Equal(t, "Great work!", resp.Feedback)

	// The classifier must have seen every user-authored field before the
	// generation model ran.
	var classified string
	for _, call := range mc.Calls() {
		if call.ModelProfile == ai.ModelClassifier && len(call.Messages) > 0 {
			classified = call.Messages[len(call.Messages)-1].Content
		}
	}
	assert.Contains(t, classified, "felt anxious")
	assert.Contains(t, classified, "skipped yesterday")
	assert.Contains(t, classified, "Meditate")
}

// A1/A2: a flagged blocker or note returns the deterministic response and
// never reaches the generation model.
func TestGenerateCheckInFeedback_BlockedNoteReturnsDeterministic(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `{"category":"eating_disorder","confidence":0.9,"reason":"restriction talk"}`,
	}, ai.Usage{}, 0)
	// No ModelCheap response recorded — the model must NOT be called.

	svcCtx := &svc.ServiceContext{AIClient: mc, Classifier: safety.NewLLMClassifier(mc)}
	logic := NewGenerateCheckInFeedbackLogic(context.Background(), svcCtx)

	resp, err := logic.GenerateCheckInFeedback(&aicoach.CheckInFeedbackRequest{
		UserId:    "user-1",
		HabitName: "Eat well",
		Note:      "planning to skip meals all week",
	})
	require.NoError(t, err)
	assert.Equal(t, safety.EatingDisorderResponse, resp.Feedback)
}

func TestGenerateCheckInFeedback_ClassifierErrorFailsClosed(t *testing.T) {
	mc := aitest.NewMockClient()
	// No ModelClassifier response → classifier errors on both attempts. A
	// ModelCheap response IS recorded so proceeding would produce "Great
	// work!" — the fail-closed path must return the generic fallback instead.
	mc.RecordResponse(ai.ModelCheap, ai.Message{
		Role:    ai.RoleAssistant,
		Content: "Great work!",
	}, ai.Usage{}, 0)

	svcCtx := &svc.ServiceContext{AIClient: mc, Classifier: safety.NewLLMClassifier(mc)}
	logic := NewGenerateCheckInFeedbackLogic(context.Background(), svcCtx)

	resp, err := logic.GenerateCheckInFeedback(&aicoach.CheckInFeedbackRequest{
		UserId: "user-1",
		Note:   "anything",
	})
	require.NoError(t, err)
	assert.Equal(t, "Keep showing up — every check-in counts.", resp.Feedback)
}
