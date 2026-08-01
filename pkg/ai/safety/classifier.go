package safety

import "context"

// Category classifies the safety verdict of user input.
type Category string

const (
	// CategorySafe means the input poses no safety concern.
	CategorySafe Category = "safe"
	// CategoryCrisis means the user may be in immediate danger.
	CategoryCrisis Category = "crisis"
	// CategoryMedical means the input seeks medical advice.
	CategoryMedical Category = "medical"
	// CategorySelfHarm means the input references self-harm.
	CategorySelfHarm Category = "self_harm"
	// CategoryViolence means the input references violence.
	CategoryViolence Category = "violence"
)

// Verdict is the result of classifying user input for safety.
type Verdict struct {
	Category   Category
	Confidence float64
	Reason     string
}

// Classifier pre-screens user input for crisis / medical advice / self-harm.
// Callers decide what to do with the verdict — pkg/ai stays policy-free.
type Classifier interface {
	Classify(ctx context.Context, text string) (Verdict, error)
}

// CrisisResponse is the deterministic, never-model-generated response streamed
// to the user when the safety classifier detects a crisis or self-harm signal.
// It is a single source of truth so the model can never hallucinate or omit
// safety resources. Region-aware helplines can be derived from the user
// profile location later; for now this covers the most common regions.
const CrisisResponse = `It sounds like you're going through something really painful right now, and I'm glad you reached out.
I'm an accountability coach, not a crisis counselor, so I want to make sure you get the right support.

If you're in immediate danger, please call your local emergency number now.
• US: call or text 988 (Suicide & Crisis Lifeline)
• UK & ROI: call 116 123 (Samaritans)
• Elsewhere: https://findahelpline.com

You deserve support from someone trained to help. Would you like to keep talking about your goals when you're ready?`
