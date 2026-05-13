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
