package memory

import "github.com/suleymanmyradov/growth-server/pkg/ai"

// ConversationWindow keeps the last N messages, dropping the oldest.
// Use it to bound the context window before sending to the model.
type ConversationWindow struct {
	maxMessages int
	messages    []ai.Message
}

// NewConversationWindow creates a window that retains at most maxMessages.
func NewConversationWindow(maxMessages int) *ConversationWindow {
	if maxMessages < 1 {
		maxMessages = 1
	}
	return &ConversationWindow{maxMessages: maxMessages}
}

// Add appends messages and trims to maxMessages, always preserving
// the first message if it is a system message.
func (w *ConversationWindow) Add(msgs ...ai.Message) {
	w.messages = append(w.messages, msgs...)

	// If we exceed the limit, trim from the front.
	if len(w.messages) <= w.maxMessages {
		return
	}

	// Preserve system message at index 0 if present.
	hasSystem := len(w.messages) > 0 && w.messages[0].Role == ai.RoleSystem
	excess := len(w.messages) - w.maxMessages

	if hasSystem && excess > 0 {
		// Keep system message + last (maxMessages-1) messages.
		w.messages = append(
			w.messages[:1],
			w.messages[1+excess:]...,
		)
	} else {
		w.messages = w.messages[excess:]
	}
}

// Messages returns the current window of messages.
func (w *ConversationWindow) Messages() []ai.Message {
	return w.messages
}

// Clear removes all messages.
func (w *ConversationWindow) Clear() {
	w.messages = w.messages[:0]
}

// Len returns the current number of messages.
func (w *ConversationWindow) Len() int {
	return len(w.messages)
}
