package ai

import "testing"

func BenchmarkDailyKey(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = dailyKey("user", "user-123")
	}
}
