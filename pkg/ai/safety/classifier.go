package safety

import (
	"context"
	"time"
)

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
	// CategoryEatingDisorder means the input references disordered-eating
	// behaviors (restriction, purging, binge/restrict cycles, compensatory
	// exercise) or asks for help designing or validating them.
	CategoryEatingDisorder Category = "eating_disorder"
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

// UnavailableResponse is the deterministic, never-model-generated response
// sent when the safety classifier cannot produce a verdict (provider error,
// timeout, or unparseable output). Callers must fail closed — the model never
// sees unscreened input — and this message both acknowledges the degraded
// state and surfaces crisis resources in case the user needs them.
const UnavailableResponse = `I'm having trouble reading your message right now, so I'd rather pause than guess. Please try again in a moment.

If you're in crisis or thinking about harming yourself, please reach out now:
• US: call or text 988 (Suicide & Crisis Lifeline)
• UK & ROI: call 116 123 (Samaritans)
• Elsewhere: https://findahelpline.com`

// MedicalResponse is the deterministic, never-model-generated response sent
// when the user asks for medical, diagnostic, treatment, or diet-prescription
// advice the coach must not give.
const MedicalResponse = `That's an important question, but it's outside what I can safely help with — I'm a habit coach, not a clinician.

For anything involving diagnosis, treatment, medication, injuries, or a specific diet plan, please talk to a doctor or registered dietitian who can look at your full picture.

What I can help with is the habit side: sleep, movement, consistency, and routines. Want to work on one of those together?`

// EatingDisorderResponse is the deterministic, never-model-generated response
// sent when the user describes disordered-eating behaviors or asks for help
// designing them (severe restriction, purging, binge/restrict cycles,
// compensatory exercise). Supportive, non-shaming, routes to real help.
const EatingDisorderResponse = `I'm really glad you told me this, and I want to be honest with you: what you're describing deserves more support than a habit coach can give.

Please consider talking to a doctor, therapist, or eating-disorder specialist — you don't have to figure this out alone.
• US: call or text 800-931-2237 (NEDA)
• Elsewhere: https://findahelpline.com

I'm still here for the everyday stuff — routines, rest, and habits that support you.`

// DeclineResponse is the deterministic, never-model-generated response sent
// when input is flagged for a non-crisis category without a tailored reply
// (e.g. violence toward others, or an unrecognized flagged category). It
// declines without lecturing and redirects to coaching scope.
const DeclineResponse = `I'm not able to help with that. I'm a habit and accountability coach — if you'd like, we can talk about your goals, routines, or what's getting in the way.`

// BlockConfidenceThreshold is the shared minimum classifier confidence at
// which a flagged verdict is blocked with a deterministic response. Below it
// the input still reaches the model, but only under the coaching-scope
// system-prompt constraints — flag-and-proceed is deliberate, not silent:
// callers log the flag. Every AI surface uses this same threshold so policy
// doesn't drift between check-ins, coaching, reviews, and the agent.
const BlockConfidenceThreshold = 0.75

// BlockedResponse maps a flagged verdict to its deterministic user-facing
// reply. It returns ("", false) for safe input and for flagged input below
// minConfidence — callers pass their own threshold (0 means "block on any
// flagged category"). Unknown non-safe categories fail closed to
// DeclineResponse so a newer classifier label is never silently passed to
// the model.
func BlockedResponse(v Verdict, minConfidence float64) (string, bool) {
	if v.Category == CategorySafe || v.Confidence < minConfidence {
		return "", false
	}
	switch v.Category {
	case CategoryCrisis, CategorySelfHarm:
		return CrisisResponse, true
	case CategoryEatingDisorder:
		return EatingDisorderResponse, true
	case CategoryMedical:
		return MedicalResponse, true
	default:
		return DeclineResponse, true
	}
}

// classifyRetryDelay is the pause before the single classifier retry. Short
// enough to stay inside the classify timeout, long enough to ride out a
// provider blip.
const classifyRetryDelay = 300 * time.Millisecond

// ClassifyWithRetry calls c.Classify and retries once on error. A persistent
// failure is returned as an error — callers must treat that as "unknown",
// never as "safe" (fail closed).
func ClassifyWithRetry(ctx context.Context, c Classifier, text string) (Verdict, error) {
	verdict, err := c.Classify(ctx, text)
	if err == nil {
		return verdict, nil
	}
	if ctx.Err() != nil {
		return Verdict{}, err
	}
	select {
	case <-ctx.Done():
		return Verdict{}, err
	case <-time.After(classifyRetryDelay):
	}
	return c.Classify(ctx, text)
}
