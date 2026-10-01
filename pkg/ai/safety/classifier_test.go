package safety

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCategory_String(t *testing.T) {
	tests := []struct {
		cat    Category
		expect string
	}{
		{CategorySafe, "safe"},
		{CategoryCrisis, "crisis"},
		{CategoryMedical, "medical"},
		{CategorySelfHarm, "self_harm"},
		{CategoryViolence, "violence"},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.expect, string(tt.cat))
	}
}

func TestVerdict_Fields(t *testing.T) {
	v := Verdict{
		Category:   CategorySelfHarm,
		Confidence: 0.95,
		Reason:     "user mentions self-harm",
	}
	assert.Equal(t, CategorySelfHarm, v.Category)
	assert.Equal(t, 0.95, v.Confidence)
	assert.Equal(t, "user mentions self-harm", v.Reason)
}

func TestCategory_String_IncludesEatingDisorder(t *testing.T) {
	assert.Equal(t, "eating_disorder", string(CategoryEatingDisorder))
}

func TestBlockedResponse(t *testing.T) {
	tests := []struct {
		name        string
		verdict     Verdict
		threshold   float64
		wantResp    string
		wantBlocked bool
	}{
		{"safe input never blocks", Verdict{Category: CategorySafe, Confidence: 0.99}, BlockConfidenceThreshold, "", false},
		{"crisis blocks with crisis resources", Verdict{Category: CategoryCrisis, Confidence: 0.9}, BlockConfidenceThreshold, CrisisResponse, true},
		{"self_harm blocks with crisis resources", Verdict{Category: CategorySelfHarm, Confidence: 0.8}, BlockConfidenceThreshold, CrisisResponse, true},
		{"medical blocks with medical response", Verdict{Category: CategoryMedical, Confidence: 0.9}, BlockConfidenceThreshold, MedicalResponse, true},
		{"eating_disorder blocks with ED response", Verdict{Category: CategoryEatingDisorder, Confidence: 0.85}, BlockConfidenceThreshold, EatingDisorderResponse, true},
		{"violence declines", Verdict{Category: CategoryViolence, Confidence: 0.9}, BlockConfidenceThreshold, DeclineResponse, true},
		{"unknown category fails closed to decline", Verdict{Category: "jailbreak", Confidence: 0.9}, BlockConfidenceThreshold, DeclineResponse, true},
		{"flag below threshold passes to model", Verdict{Category: CategoryMedical, Confidence: 0.4}, BlockConfidenceThreshold, "", false},
		{"flag at threshold blocks", Verdict{Category: CategoryMedical, Confidence: BlockConfidenceThreshold}, BlockConfidenceThreshold, MedicalResponse, true},
		{"zero threshold blocks any flag", Verdict{Category: CategoryMedical, Confidence: 0.1}, 0, MedicalResponse, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, blocked := BlockedResponse(tt.verdict, tt.threshold)
			assert.Equal(t, tt.wantBlocked, blocked)
			assert.Equal(t, tt.wantResp, resp)
		})
	}
}
