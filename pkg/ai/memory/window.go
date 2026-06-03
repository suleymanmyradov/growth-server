package memory

import (
	"strings"
	"sync"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
)

// ConversationWindow keeps the last N messages, dropping the oldest.
// Use it to bound the context window before sending to the model.
// Safe for concurrent use.
type ConversationWindow struct {
	mu          sync.RWMutex
	maxMessages int
	maxTokens   int
	messages    []ai.Message
}

// NewConversationWindow creates a window that retains at most maxMessages.
func NewConversationWindow(maxMessages int) *ConversationWindow {
	if maxMessages < 1 {
		maxMessages = 1
	}
	return &ConversationWindow{maxMessages: maxMessages}
}

// NewConversationWindowTokens creates a window that trims messages to stay
// within maxTokens (approximate). When maxTokens > 0, maxMessages is ignored.
func NewConversationWindowTokens(maxTokens int) *ConversationWindow {
	if maxTokens < 1 {
		maxTokens = 1
	}
	return &ConversationWindow{maxTokens: maxTokens}
}

// Add appends messages and trims, preserving the system message if present.
// Trimming is by message count (maxMessages) unless maxTokens is set,
// in which case it drops oldest messages until the total is under budget.
func (w *ConversationWindow) Add(msgs ...ai.Message) {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.messages = append(w.messages, msgs...)

	if w.maxTokens > 0 {
		w.trimByTokens()
		return
	}

	w.trimByCount()
}

func (w *ConversationWindow) trimByCount() {
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

func (w *ConversationWindow) trimByTokens() {
	for {
		total := approximateTokens(w.messages)
		if total <= w.maxTokens {
			return
		}
		if len(w.messages) <= 1 {
			return
		}
		// Drop the oldest non-system message, or the oldest message if
		// there's no system message.
		if len(w.messages) > 1 && w.messages[0].Role == ai.RoleSystem {
			w.messages = append(w.messages[:1], w.messages[2:]...)
		} else {
			w.messages = w.messages[1:]
		}
	}
}

// approximateTokens returns a rough token count for a slice of messages.
// Uses the rule of thumb: 1 token ~ 0.75 English words, so words * 1.33.
func approximateTokens(msgs []ai.Message) int {
	var words int
	for _, m := range msgs {
		words += len(strings.Fields(m.Content))
		for _, tc := range m.ToolCalls {
			words += len(strings.Fields(tc.Fn.Name))
			words += len(strings.Fields(tc.Fn.Arguments))
		}
	}
	return int(float64(words) * 1.33)
}

// Messages returns the current window of messages.
func (w *ConversationWindow) Messages() []ai.Message {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]ai.Message, len(w.messages))
	copy(out, w.messages)
	return out
}

// Clear removes all messages.
func (w *ConversationWindow) Clear() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.messages = w.messages[:0]
}

// Len returns the current number of messages.
func (w *ConversationWindow) Len() int {
	w.mu.RLock()
	defer w.mu.RUnlock()
	return len(w.messages)
}
