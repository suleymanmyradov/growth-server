package safety

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/suleymanmyradov/growth-server/pkg/ai"
)

const classifierSystemPrompt = `You are a conservative safety classifier for a habit and accountability coach. Your job is to catch messages that need an immediate crisis or self-harm response, and requests that ask the coach to act as a clinician or to design unsafe regimens. Do not over-flag ordinary stress, anxiety, sadness, overwhelm, exhaustion, work pressure, financial pressure, smoking, or fear of failure as self_harm or crisis unless the user explicitly mentions wanting to die, hurt themselves, or being in immediate danger.

Classify the user input into exactly one of these categories:
- safe: No safety concern. This includes normal expressions of stress, anxiety, low mood, overwhelm, tiredness, smoking relapse, financial pressure, feeling like a failure, and ordinary fitness, diet, or weight goals (e.g. "lose 5 kg", "eat healthier", "run a marathon", "go to the gym daily") — as long as there is no explicit self-harm, suicidal ideation, immediate danger, or unsafe regimen.
- crisis: The user is in immediate danger or explicitly states intent to harm themselves or others right now.
- medical: The user seeks a medical diagnosis, treatment plan, medication advice, supplement prescription, or a specific prescribed diet — OR asks the coach to design/validate an extreme or unsafe regimen, such as prolonged fasting, very-low-calorie or starvation diets, purging, exercising through acute injury, or rapid weight-loss targets far beyond safe rates. Ordinary healthy-eating or fitness questions are safe.
- eating_disorder: The user references eating-disorder behaviors — severe restriction, purging, binge/restrict cycles, compulsive exercise to "earn" or compensate for food, or fear of weight gain driving harmful restriction — or asks for help doing any of these. Ordinary calorie tracking, balanced diets, or fitness goals are safe.
- self_harm: The user explicitly references self-injury, self-harm, or suicidal ideation or intent. Vague negative self-talk (e.g. "I feel unsuccessful") is NOT self_harm.
- violence: The user is threatening or describing violence toward others.

Respond with ONLY a JSON object with these fields:
{"category": "<one of: safe, crisis, medical, eating_disorder, self_harm, violence>", "confidence": <0.0-1.0>, "reason": "<brief explanation>"}

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
		// Unparseable output means the classifier cannot be trusted — return
		// an error so callers fail closed instead of silently treating
		// potentially-flagged input as safe.
		return Verdict{}, fmt.Errorf("safety.Classify: unparseable classifier output %q: %w", content, err)
	}

	return Verdict{
		Category:   Category(result.Category),
		Confidence: result.Confidence,
		Reason:     result.Reason,
	}, nil
}

func floatPtr(f float32) *float32 { return &f }
func intPtr(i int) *int           { return &i }
