package memory

import (
	"context"
	"testing"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/pkg/ai/aitest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSummarize_NewSummary(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: "User discussed their fitness goals and progress this week.",
	}, ai.Usage{}, 0)

	s := NewSummarizer(mc)
	summary, err := s.Summarize(context.Background(), []ai.Message{
		{Role: ai.RoleUser, Content: "I want to run 5k"},
		{Role: ai.RoleAssistant, Content: "Great goal!"},
	}, "")
	require.NoError(t, err)
	assert.Contains(t, summary, "fitness goals")
}

func TestSummarize_ExistingSummary(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.RecordResponse(ai.ModelClassifier, ai.Message{
		Role:    ai.RoleAssistant,
		Content: "Updated summary incorporating new messages.",
	}, ai.Usage{}, 0)

	s := NewSummarizer(mc)
	summary, err := s.Summarize(context.Background(), []ai.Message{
		{Role: ai.RoleUser, Content: "I ran 3k today"},
	}, "User wants to run 5k.")
	require.NoError(t, err)
	assert.Contains(t, summary, "Updated summary")
}

func TestSummarize_Error(t *testing.T) {
	mc := aitest.NewMockClient()
	mc.SetError(ai.ErrModelUnavailable)

	s := NewSummarizer(mc)
	_, err := s.Summarize(context.Background(), []ai.Message{
		{Role: ai.RoleUser, Content: "test"},
	}, "")
	assert.Error(t, err)
}
