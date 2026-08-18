package ai

import "testing"

func BenchmarkDailyKey(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		_ = dailyKey("user", "user-123")
	}
}
