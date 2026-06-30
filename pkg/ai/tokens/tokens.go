// Package tokens provides token counting for prompt budgeting.
//
// It uses tiktoken-go's cl100k_base encoding as a single, model-agnostic
// approximation. cl100k_base is close enough for budgeting across the GPT-4 /
// GPT-4o / o-family models we route to; exact per-model counts are not needed
// for "fill a budget then stop" logic. If the encoder is unavailable for any
// reason, Count falls back to a runes/4 heuristic so budgeting never breaks.
package tokens

import (
	"sync"

	"github.com/tiktoken-go/tokenizer"
)

var (
	once    sync.Once
	encoder tokenizer.Codec
)

// initEncoder lazily builds the cl100k_base codec. Codec construction loads the
// vocab, so we do it at most once per process.
func initEncoder() {
	once.Do(func() {
		enc, err := tokenizer.Get(tokenizer.Cl100kBase)
		if err != nil {
			// Leave encoder nil; Count will fall back to the heuristic.
			return
		}
		encoder = enc
	})
}

// Count returns the approximate token count of s. It uses the cl100k_base
// tokenizer when available and falls back to a runes/4 heuristic otherwise.
// The result is always >= 0 and never panics on invalid UTF-8.
func Count(s string) int {
	if s == "" {
		return 0
	}
	initEncoder()
	if encoder != nil {
		if n, err := encoder.Count(s); err == nil && n >= 0 {
			return n
		}
	}
	return Estimate(s)
}

// Estimate is a dependency-free heuristic (runes/4, rounded up) used as a
// fallback when the real tokenizer is unavailable. It is intentionally
// conservative for budgeting: it slightly under-counts dense English and
// over-counts whitespace-heavy text, which is acceptable for "fill a budget"
// logic where the budget itself has headroom.
func Estimate(s string) int {
	if s == "" {
		return 0
	}
	n := len([]rune(s)) / 4
	if n == 0 {
		n = 1
	}
	return n
}
