package memory

import (
	"testing"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/stretchr/testify/assert"
)

func TestConversationWindow_Add(t *testing.T) {
	w := NewConversationWindow(3)

	w.Add(
		ai.Message{Role: ai.RoleSystem, Content: "system"},
		ai.Message{Role: ai.RoleUser, Content: "hello"},
		ai.Message{Role: ai.RoleAssistant, Content: "hi"},
		ai.Message{Role: ai.RoleUser, Content: "how are you"},
	)

	msgs := w.Messages()
	// Should keep system + last 2 non-system messages.
	assert.Equal(t, 3, len(msgs))
	assert.Equal(t, ai.RoleSystem, msgs[0].Role)
	assert.Equal(t, "hi", msgs[1].Content)
	assert.Equal(t, "how are you", msgs[2].Content)
}

func TestConversationWindow_NoSystem(t *testing.T) {
	w := NewConversationWindow(2)

	w.Add(
		ai.Message{Role: ai.RoleUser, Content: "a"},
		ai.Message{Role: ai.RoleAssistant, Content: "b"},
		ai.Message{Role: ai.RoleUser, Content: "c"},
	)

	msgs := w.Messages()
	assert.Equal(t, 2, len(msgs))
	assert.Equal(t, "b", msgs[0].Content)
	assert.Equal(t, "c", msgs[1].Content)
}

func TestConversationWindow_Clear(t *testing.T) {
	w := NewConversationWindow(5)
	w.Add(ai.Message{Role: ai.RoleUser, Content: "hello"})
	assert.Equal(t, 1, w.Len())

	w.Clear()
	assert.Equal(t, 0, w.Len())
}

func TestConversationWindow_SmallWindow(t *testing.T) {
	w := NewConversationWindow(0) // should default to 1
	assert.Equal(t, 1, w.maxMessages)

	w.Add(
		ai.Message{Role: ai.RoleUser, Content: "a"},
		ai.Message{Role: ai.RoleUser, Content: "b"},
	)
	assert.Equal(t, 1, w.Len())
}
