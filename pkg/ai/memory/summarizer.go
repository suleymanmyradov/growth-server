package memory

import (
	"context"
	"fmt"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
)

// Summarizer compresses long conversation history by calling Generate
// with the classifier model to produce a running summary. Used to
// compress long conversations before sending to the chat model.
type Summarizer struct {
	client ai.Client
}

// NewSummarizer creates a new Summarizer.
func NewSummarizer(client ai.Client) *Summarizer {
	return &Summarizer{client: client}
}

// Summarize produces a summary of the given messages.
// It uses the ModelClassifier profile for cost efficiency.
func (s *Summarizer) Summarize(ctx context.Context, messages []ai.Message, existingSummary string) (string, error) {
	var prompt string
	if existingSummary != "" {
		prompt = fmt.Sprintf(
			"Below is an existing summary of a conversation, followed by new messages.\n"+
				"Produce an updated summary that incorporates the new messages.\n\n"+
				"EXISTING SUMMARY:\n%s\n\nNEW MESSAGES:\n%s\n\nUPDATED SUMMARY:",
			existingSummary,
			formatMessages(messages),
		)
	} else {
		prompt = fmt.Sprintf(
			"Summarize the following conversation concisely, preserving key facts, decisions, and context:\n\n%s\n\nSUMMARY:",
			formatMessages(messages),
		)
	}

	resp, err := s.client.Generate(ctx, ai.GenerateRequest{
		ModelProfile: ai.ModelClassifier,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: prompt},
		},
		Temperature: floatPtr(0.3),
		MaxTokens:   intPtr(500),
	})
	if err != nil {
		return "", fmt.Errorf("memory.Summarize: %w", err)
	}

	return resp.Message.Content, nil
}

func formatMessages(msgs []ai.Message) string {
	var out string
	for _, m := range msgs {
		out += fmt.Sprintf("[%s]: %s\n", m.Role, m.Content)
	}
	return out
}

func floatPtr(f float32) *float32 { return &f }
func intPtr(i int) *int           { return &i }
