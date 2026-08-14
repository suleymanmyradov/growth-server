package ai

import (
	"regexp"
	"strings"
)

// thoughtTagRegex matches complete <thought>...</thought> blocks (case-insensitive,
// DOTALL so it spans newlines). Gemma-4 models emit these inline in the content
// field and thinking cannot be disabled via the API ("Thinking budget is not
// supported for this model"). We extract the thought content and route it to
// the Reasoning field so the frontend's existing "thinking" UI displays it.
var thoughtTagRegex = regexp.MustCompile(`(?is)<thought>(.*?)</thought>`)

// thoughtParts holds the result of splitting content into reasoning and output.
type thoughtParts struct {
	Reasoning string // content from inside <thought>...</thought> blocks
	Content   string // content outside <thought> blocks (the actual response)
}

// splitThoughtTags separates <thought>...</thought> blocks from regular content.
// The thought content is returned as Reasoning; the rest as Content.
// Used for non-streaming responses where the full content is available at once.
// If the opening <thought> tag has no matching closing tag (truncated response),
// treats everything from <thought> onward as reasoning.
func splitThoughtTags(s string) thoughtParts {
	var reasoning, content strings.Builder

	// Find all complete <thought>...</thought> blocks and extract them.
	loc := thoughtTagRegex.FindStringSubmatchIndex(s)
	for loc != nil {
		// Content before the thought block.
		content.WriteString(s[:loc[0]])
		// Reasoning inside the thought block (group 1).
		reasoning.WriteString(s[loc[2]:loc[3]])
		reasoning.WriteString("\n")
		// Continue after the thought block.
		s = s[loc[1]:]
		loc = thoughtTagRegex.FindStringSubmatchIndex(s)
	}

	// Check for truncated thought: <thought> without closing tag.
	if idx := strings.Index(strings.ToLower(s), "<thought>"); idx != -1 {
		// Content before the unclosed thought.
		content.WriteString(s[:idx])
		// The rest is reasoning (truncated, no closing tag).
		reasoning.WriteString(s[idx+len("<thought>"):])
	} else {
		// No thought tag — remaining is all content.
		content.WriteString(s)
	}

	return thoughtParts{
		Reasoning: strings.TrimSpace(reasoning.String()),
		Content:   strings.TrimSpace(content.String()),
	}
}

// thoughtFilter is a stateful filter for streaming responses. It separates
// content inside <thought>...</thought> blocks (routing it as reasoning) from
// content outside the blocks (routing it as delta). This lets the frontend's
// existing reasoning/thinking UI display Gemma-4's thinking process.
//
// Usage:
//
//	f := newThoughtFilter()
//	for /* each delta */ {
//	    parts := f.filter(delta)
//	    if parts.Reasoning != "" { /* forward as reasoning event */ }
//	    if parts.Content != ""   { /* forward as delta event */ }
//	}
//	// At end of stream:
//	remaining := f.flush() // forward any buffered content
type thoughtFilter struct {
	buf       strings.Builder // buffers content while inside <thought>
	inThought bool            // true once we've seen <thought> and not yet </thought>
	started   bool            // true once we've determined whether this stream uses thoughts
}

func newThoughtFilter() *thoughtFilter {
	return &thoughtFilter{}
}

// filter processes a content delta and returns the reasoning and content
// portions that should be forwarded to the client.
func (f *thoughtFilter) filter(delta string) thoughtParts {
	if delta == "" {
		return thoughtParts{}
	}

	// If we haven't determined yet whether this stream uses thought tags,
	// check if the delta starts with (or contains) <thought>.
	if !f.started {
		lower := strings.ToLower(delta)
		if strings.HasPrefix(lower, "<thought>") {
			f.started = true
			f.inThought = true
			// Buffer everything after <thought> in this delta.
			after := delta[len("<thought>"):]
			f.buf.WriteString(after)
			return f.drainThoughtBuffer()
		}
		// No thought tag at the start — this model doesn't use thoughts.
		// Forward everything as content.
		f.started = true
		return thoughtParts{Content: delta}
	}

	// Already started: if not in thought mode, forward directly as content.
	if !f.inThought {
		return thoughtParts{Content: delta}
	}

	// In thought mode: buffer and look for </thought>.
	f.buf.WriteString(delta)
	return f.drainThoughtBuffer()
}

// drainThoughtBuffer processes the buffered content while in thought mode.
// It looks for </thought> in the buffer. If found, it switches to output mode
// and returns any reasoning/content. If not found, returns empty parts.
func (f *thoughtFilter) drainThoughtBuffer() thoughtParts {
	buf := f.buf.String()
	lowerBuf := strings.ToLower(buf)
	closeIdx := strings.Index(lowerBuf, "</thought>")
	if closeIdx == -1 {
		// Still inside thought — keep buffering, forward nothing.
		return thoughtParts{}
	}

	// Found closing tag. Switch to output mode.
	f.inThought = false
	reasoning := buf[:closeIdx]
	after := buf[closeIdx+len("</thought>"):]
	f.buf.Reset()

	var parts thoughtParts
	parts.Reasoning = reasoning

	// The content after </thought> might itself contain new thought tags
	// (unlikely but possible). Check for it.
	if after == "" {
		return parts
	}
	lowerAfter := strings.ToLower(after)
	if strings.HasPrefix(lowerAfter, "<thought>") {
		// Nested thought — re-enter thought mode.
		f.inThought = true
		f.buf.WriteString(after[len("<thought>"):])
		inner := f.drainThoughtBuffer()
		parts.Reasoning += inner.Reasoning
		parts.Content = inner.Content
		return parts
	}
	parts.Content = after
	return parts
}

// flush returns any remaining buffered content when the stream ends.
// This handles the case where a thought block was never closed (truncated).
func (f *thoughtFilter) flush() thoughtParts {
	if !f.inThought {
		// Not in thought mode — any buffered content is real output.
		s := f.buf.String()
		f.buf.Reset()
		if s == "" {
			return thoughtParts{}
		}
		return thoughtParts{Content: s}
	}
	// In thought mode at end of stream — the thought was never closed.
	// Return the buffered content as reasoning (truncated thinking).
	s := f.buf.String()
	f.buf.Reset()
	return thoughtParts{Reasoning: s}
}
