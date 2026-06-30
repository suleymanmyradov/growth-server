package weeklyreviewservicelogic

import "testing"

func BenchmarkLoadLocationCached(b *testing.B) {
	// Warm cache with one call.
	_, _ = loadLocationCached("America/New_York")

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = loadLocationCached("America/New_York")
	}
}
