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
