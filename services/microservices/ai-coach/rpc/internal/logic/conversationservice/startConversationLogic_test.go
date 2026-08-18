package conversationservicelogic

import (
	"strings"
	"testing"
)

func TestTruncateTitle(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		maxLen  int
		want    string
	}{
		{
			name:   "empty stays empty",
			input:  "",
			maxLen: 252,
			want:   "",
		},
		{
			name:   "short title unchanged",
			input:  "I'm tired of vibe coding",
			maxLen: 252,
			want:   "I'm tired of vibe coding",
		},
		{
			name:   "exactly maxLen unchanged",
			input:  strings.Repeat("a", 252),
			maxLen: 252,
			want:   strings.Repeat("a", 252),
		},
		{
			name:   "over maxLen truncated with ellipsis",
			input:  strings.Repeat("a", 600),
			maxLen: 252,
			want:   strings.Repeat("a", 249) + "...",
		},
		{
			name:   "multi-line collapsed to single line",
			input:  "line one\n\nline two\ttabbed   spaces",
			maxLen: 252,
			want:   "line one line two tabbed spaces",
		},
		{
			name:   "leading/trailing whitespace trimmed",
			input:  "  hello world  ",
			maxLen: 252,
			want:   "hello world",
		},
		{
			name:   "long multi-line message truncated after collapse",
			input:  "I'm a Full stack web developer\n\nand after AI models released\n\nour productivity grow dramatically",
			maxLen: 30,
			want:   "I'm a Full stack web develo...",
		},
		{
			name:   "very small maxLen",
			input:  "hello",
			maxLen: 3,
			want:   "hel",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateTitle(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateTitle(%q, %d) = %q (len=%d), want %q (len=%d)",
					tt.input, tt.maxLen, got, len(got), tt.want, len(tt.want))
			}
			// Verify the result never exceeds maxLen.
			if len(got) > tt.maxLen {
				t.Errorf("result length %d exceeds maxLen %d", len(got), tt.maxLen)
			}
		})
	}
}

// TestTruncateTitleFitsVarchar255 verifies that the title produced by
// truncateTitle always fits in a varchar(255) column when called with the
// production maxLen of 252. This guards against the SQLSTATE 22001 bug
// where long opening messages exceeded the column limit.
func TestTruncateTitleFitsVarchar255(t *testing.T) {
	longMessage := "I'm a Full stack web developer and after AI models released our productivity grow dramatically but When you need to hold too much thing on memory test what I did, say what need to fix, all recursive tasks, vibe coding staff really makes me crazy nervous.\n\nI'm not a smoker but I tried it several weeks or months ago, and that anaxitie and nervous feeling makes to smoke again, also I can't rest when I'm not doing stuff, I can't watch movies or play games and enjoy it like before, everything feels so fast, and I have a feeling I will be late for the train, and I will be unsuccessful person in life"

	title := truncateTitle(longMessage, 252)
	if len(title) > 255 {
		t.Fatalf("title length %d exceeds varchar(255): %q", len(title), title)
	}
	if !strings.HasSuffix(title, "...") {
		t.Fatalf("expected truncated title to end with '...', got: %q", title)
	}
}
