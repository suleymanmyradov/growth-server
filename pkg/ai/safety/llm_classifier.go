package safety

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
)

const classifierSystemPrompt = `You are a conservative safety classifier. Your job is to catch messages that need an immediate crisis or self-harm response. Do not over-flag ordinary stress, anxiety, sadness, overwhelm, exhaustion, work pressure, financial pressure, smoking, or fear of failure as self_harm or crisis unless the user explicitly mentions wanting to die, hurt themselves, or being in immediate danger.

Classify the user input into exactly one of these categories:
- safe: No safety concern. This includes normal expressions of stress, anxiety, low mood, overwhelm, tiredness, smoking relapse, financial pressure, or feeling like a failure — as long as there is no explicit self-harm, suicidal ideation, or immediate danger.
- crisis: The user is in immediate danger or explicitly states intent to harm themselves or others right now.
- medical: The user is seeking specific medical diagnosis or treatment advice.
- self_harm: The user explicitly references self-injury, self-harm, or suicidal ideation or intent. Vague negative self-talk (e.g. "I feel unsuccessful") is NOT self_harm.
- violence: The user is threatening or describing violence toward others.

Respond with ONLY a JSON object with these fields:
{"category": "<one of: safe, crisis, medical, self_harm, violence>", "confidence": <0.0-1.0>, "reason": "<brief explanation>"}

When in doubt, choose safe. High confidence (>=0.85) should only be assigned when the evidence is explicit and unambiguous. Never refuse to classify. Always respond with the JSON.`

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
func intPtr(i int) *int           { return &i }
