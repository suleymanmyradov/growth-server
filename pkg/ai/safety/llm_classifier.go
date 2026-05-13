package safety

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
)

const classifierSystemPrompt = `You are a safety classifier. Given user input, classify it into exactly one of these categories:
- safe: No safety concern.
- crisis: User may be in immediate danger or expressing intent to harm others.
- medical: User is seeking medical advice or describing medical symptoms.
- self_harm: User is referencing self-harm or suicidal ideation.
- violence: User is referencing or threatening violence.

Respond with ONLY a JSON object with these fields:
{"category": "<one of: safe, crisis, medical, self_harm, violence>", "confidence": <0.0-1.0>, "reason": "<brief explanation>"}

Never refuse to classify. Always respond with the JSON.`

// LLMClassifier uses the classifier model to classify user input.
type LLMClassifier struct {
	client ai.Client
}

// NewLLMClassifier creates a classifier backed by the AI client.
func NewLLMClassifier(client ai.Client) *LLMClassifier {
	return &LLMClassifier{client: client}
}

// classifyResult is the JSON structure returned by the classifier model.
type classifyResult struct {
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Reason     string  `json:"reason"`
}

// Classify classifies user input for safety concerns.
func (c *LLMClassifier) Classify(ctx context.Context, text string) (Verdict, error) {
	resp, err := c.client.Generate(ctx, ai.GenerateRequest{
		ModelProfile: ai.ModelClassifier,
		System:       classifierSystemPrompt,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: text},
		},
		Temperature:    floatPtr(0.0),
		MaxTokens:      intPtr(200),
		ResponseFormat: ai.ResponseFormatJSON,
	})
	if err != nil {
		return Verdict{}, fmt.Errorf("safety.Classify: %w", err)
	}

	content := strings.TrimSpace(resp.Message.Content)
	// Strip markdown code fences if present.
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	var result classifyResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		// If we can't parse, default to safe with low confidence.
		return Verdict{
			Category:   CategorySafe,
			Confidence: 0.0,
			Reason:     "classifier output unparseable",
		}, nil
	}

	return Verdict{
		Category:   Category(result.Category),
		Confidence: result.Confidence,
		Reason:     result.Reason,
	}, nil
}

func floatPtr(f float32) *float32 { return &f }
func intPtr(i int) *int            { return &i }
