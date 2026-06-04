package consumer

import (
	"testing"

	"github.com/suleymanmyradov/growth-server/pkg/ai/safety"
)

func BenchmarkSafetyCache(b *testing.B) {
	h := &EventsHandler{}
	h.safetyCache.Store("running", safety.Verdict{Category: safety.CategorySafe, Confidence: 0.99})

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if v, ok := h.safetyCache.Load("running"); ok {
			_ = v.(safety.Verdict)
		}
	}
}
