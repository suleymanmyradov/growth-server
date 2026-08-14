package ai

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSplitThoughtTags tests separating <thought>...</thought> blocks from
// regular content, as emitted by Gemma-4 models. The thought content should
// be routed to Reasoning (for the frontend's thinking UI), not stripped.
func TestSplitThoughtTags(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantContent  string
		wantReason   string
		wantReasonOk bool // if true, just check reasoning is non-empty
	}{
		{
			name:         "simple thought + output",
			input:        `<thought>The user wants a single-word reply: "pong".</thought>pong`,
			wantContent:  "pong",
			wantReason:   `The user wants a single-word reply: "pong".`,
			wantReasonOk: false,
		},
		{
			name:         "multiline thought",
			input:        "<thought>\n* Input: \"Reply with pong\"\n* Constraint: single word\n</thought>\npong",
			wantContent:  "pong",
			wantReason:   "* Input: \"Reply with pong\"\n* Constraint: single word",
			wantReasonOk: false,
		},
		{
			name:         "no thought tags",
			input:        "just a normal response",
			wantContent:  "just a normal response",
			wantReason:   "",
			wantReasonOk: false,
		},
		{
			name:         "empty content",
			input:        "",
			wantContent:  "",
			wantReason:   "",
			wantReasonOk: false,
		},
		{
			name:         "only thought (truncated, no closing tag)",
			input:        "<thought>The user wants pong but the response was cut off",
			wantContent:  "",
			wantReason:   "The user wants pong but the response was cut off",
			wantReasonOk: false,
		},
		{
			name:         "thought with closing but no output after",
			input:        "<thought>thinking only</thought>",
			wantContent:  "",
			wantReason:   "thinking only",
			wantReasonOk: false,
		},
		{
			name:         "case insensitive tags",
			input:        "<THOUGHT>thinking</THOUGHT>output",
			wantContent:  "output",
			wantReason:   "thinking",
			wantReasonOk: false,
		},
		{
			name:         "multiple thought blocks",
			input:        "<thought>first thought</thought>middle<thought>second thought</thought>end",
			wantContent:  "middleend",
			wantReason:   "first thought\nsecond thought",
			wantReasonOk: false,
		},
		{
			name:         "output with json after thought",
			input:        `<thought>analyzing the request</thought>{"summary":"good","blockers":"none"}`,
			wantContent:  `{"summary":"good","blockers":"none"}`,
			wantReason:   "analyzing the request",
			wantReasonOk: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitThoughtTags(tt.input)
			assert.Equal(t, tt.wantContent, got.Content)
			if tt.wantReasonOk {
				assert.NotEmpty(t, got.Reasoning)
			} else {
				assert.Equal(t, tt.wantReason, got.Reasoning)
			}
		})
	}
}

// TestThoughtFilter_Streaming tests the stateful thought filter that
// routes <thought> content to Reasoning and the rest to Content across
// stream deltas.
func TestThoughtFilter_Streaming(t *testing.T) {
	t.Run("no thought tags - passthrough as content", func(t *testing.T) {
		f := newThoughtFilter()
		parts := f.filter("hello ")
		assert.Equal(t, "hello ", parts.Content)
		assert.Empty(t, parts.Reasoning)
		parts = f.filter("world")
		assert.Equal(t, "world", parts.Content)
		assert.Empty(t, parts.Reasoning)
		parts = f.flush()
		assert.Empty(t, parts.Content)
		assert.Empty(t, parts.Reasoning)
	})

	t.Run("thought in single delta", func(t *testing.T) {
		f := newThoughtFilter()
		parts := f.filter("<thought>thinking</thought>pong")
		assert.Equal(t, "pong", parts.Content)
		assert.Equal(t, "thinking", parts.Reasoning)
		parts = f.flush()
		assert.Empty(t, parts.Content)
		assert.Empty(t, parts.Reasoning)
	})

	t.Run("thought split across deltas", func(t *testing.T) {
		f := newThoughtFilter()
		// <thought> arrives in first delta
		parts := f.filter("<thought>")
		assert.Empty(t, parts.Content)
		assert.Empty(t, parts.Reasoning)
		// thinking content
		parts = f.filter("the user wants ")
		assert.Empty(t, parts.Content)
		assert.Empty(t, parts.Reasoning)
		parts = f.filter("a single word reply")
		assert.Empty(t, parts.Content)
		assert.Empty(t, parts.Reasoning)
		// closing tag + output
		parts = f.filter("</thought>pong")
		assert.Equal(t, "pong", parts.Content)
		assert.Contains(t, parts.Reasoning, "the user wants")
		assert.Contains(t, parts.Reasoning, "a single word reply")
		parts = f.flush()
		assert.Empty(t, parts.Content)
		assert.Empty(t, parts.Reasoning)
	})

	t.Run("thought and output split across deltas", func(t *testing.T) {
		f := newThoughtFilter()
		parts := f.filter("<thought>thinking")
		assert.Empty(t, parts.Content)
		assert.Empty(t, parts.Reasoning)
		parts = f.filter(" more thinking")
		assert.Empty(t, parts.Content)
		assert.Empty(t, parts.Reasoning)
		parts = f.filter("</thought>po")
		assert.Equal(t, "po", parts.Content)
		assert.Contains(t, parts.Reasoning, "thinking")
		assert.Contains(t, parts.Reasoning, "more thinking")
		parts = f.filter("ng")
		assert.Equal(t, "ng", parts.Content)
		assert.Empty(t, parts.Reasoning)
		parts = f.flush()
		assert.Empty(t, parts.Content)
		assert.Empty(t, parts.Reasoning)
	})

	t.Run("truncated thought - no closing tag", func(t *testing.T) {
		f := newThoughtFilter()
		parts := f.filter("<thought>thinking")
		assert.Empty(t, parts.Content)
		assert.Empty(t, parts.Reasoning)
		parts = f.filter(" more thinking")
		assert.Empty(t, parts.Content)
		assert.Empty(t, parts.Reasoning)
		// Stream ends without closing tag — buffered content is reasoning
		parts = f.flush()
		assert.Empty(t, parts.Content)
		assert.Contains(t, parts.Reasoning, "thinking")
		assert.Contains(t, parts.Reasoning, "more thinking")
	})

	t.Run("empty deltas", func(t *testing.T) {
		f := newThoughtFilter()
		parts := f.filter("")
		assert.Empty(t, parts.Content)
		assert.Empty(t, parts.Reasoning)
		parts = f.filter("hello")
		assert.Equal(t, "hello", parts.Content)
		assert.Empty(t, parts.Reasoning)
		parts = f.filter("")
		assert.Empty(t, parts.Content)
		assert.Empty(t, parts.Reasoning)
		parts = f.flush()
		assert.Empty(t, parts.Content)
		assert.Empty(t, parts.Reasoning)
	})

	t.Run("output only after thought", func(t *testing.T) {
		f := newThoughtFilter()
		parts := f.filter("<thought>think</thought>")
		assert.Empty(t, parts.Content)
		assert.Equal(t, "think", parts.Reasoning)
		parts = f.filter("hello")
		assert.Equal(t, "hello", parts.Content)
		assert.Empty(t, parts.Reasoning)
		parts = f.filter(" world")
		assert.Equal(t, " world", parts.Content)
		assert.Empty(t, parts.Reasoning)
		parts = f.flush()
		assert.Empty(t, parts.Content)
		assert.Empty(t, parts.Reasoning)
	})
}
