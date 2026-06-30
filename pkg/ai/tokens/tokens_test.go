package tokens

import (
	"strings"
	"testing"
)

func TestCountEmpty(t *testing.T) {
	if got := Count(""); got != 0 {
		t.Fatalf("Count(\"\") = %d, want 0", got)
	}
}

func TestCountPositive(t *testing.T) {
	got := Count("The quick brown fox jumps over the lazy dog.")
	if got <= 0 {
		t.Fatalf("Count of a real sentence should be positive, got %d", got)
	}
}

func TestCountMonotonic(t *testing.T) {
	short := "hello world"
	long := strings.Repeat("hello world ", 100)
	if Count(long) <= Count(short) {
		t.Fatalf("longer text should have >= tokens: short=%d long=%d", Count(short), Count(long))
	}
}

func TestEstimate(t *testing.T) {
	if got := Estimate(""); got != 0 {
		t.Fatalf("Estimate(\"\") = %d, want 0", got)
	}
	if got := Estimate("ab"); got != 1 {
		t.Fatalf("Estimate(\"ab\") = %d, want 1", got)
	}
}

func TestCountFallbackSafe(t *testing.T) {
	// Invalid UTF-8 must not panic and must return a non-negative count.
	got := Count("\xff\xfe\xfd")
	if got < 0 {
		t.Fatalf("Count on invalid UTF-8 returned negative: %d", got)
	}
}
