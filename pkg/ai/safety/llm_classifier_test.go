package safety

import (
	"context"
	"testing"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/aitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLLMClassifier_Safe(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `{"category":"safe","confidence":0.95,"reason":"no safety concern"}`,
	}, ai.Usage{PromptTokens: 10, CompletionTokens: 5, TotalTokens: 15}, 0.00001)

	classifier := NewLLMClassifier(mc)
	verdict, err := classifier.Classify(context.Background(), "I had a great workout today!")
	require.NoError(t, err)
	assert.Equal(t, CategorySafe, verdict.Category)
	assert.InDelta(t, 0.95, verdict.Confidence, 0.01)
}

func TestLLMClassifier_Crisis(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: `{"category":"crisis","confidence":0.88,"reason":"user expresses distress"}`,
	}, ai.Usage{}, 0)

	classifier := NewLLMClassifier(mc)
	verdict, err := classifier.Classify(context.Background(), "I can't take this anymore")
	require.NoError(t, err)
	assert.Equal(t, CategoryCrisis, verdict.Category)
}

func TestLLMClassifier_MarkdownFence(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: "```json\n{\"category\":\"safe\",\"confidence\":0.9,\"reason\":\"ok\"}\n```",
	}, ai.Usage{}, 0)

	classifier := NewLLMClassifier(mc)
	verdict, err := classifier.Classify(context.Background(), "Hello")
	require.NoError(t, err)
	assert.Equal(t, CategorySafe, verdict.Category)
	assert.InDelta(t, 0.9, verdict.Confidence, 0.01)
}

func TestLLMClassifier_UnparseableOutput(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: "This is not JSON at all",
	}, ai.Usage{}, 0)

	classifier := NewLLMClassifier(mc)
	verdict, err := classifier.Classify(context.Background(), "test")
	require.NoError(t, err)
	assert.Equal(t, CategorySafe, verdict.Category) // defaults to safe
	assert.Equal(t, 0.0, verdict.Confidence)
}

func TestLLMClassifier_Error(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.SetError(ai.ErrModelUnavailable)

	classifier := NewLLMClassifier(mc)
	_, err := classifier.Classify(context.Background(), "test")
	assert.Error(t, err)
}
