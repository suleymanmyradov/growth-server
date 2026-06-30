package aicoachservicelogic

import (
	"context"
	"strings"
	"time"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/prompts"
	aicoach "github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"
)

// defaultRetrievalTimeout is the tight fail-open budget for a Meili query.
const defaultRetrievalTimeout = 300 * time.Millisecond

// retrieveMemories fetches cross-conversation memory snippets for the user's
// message, de-dupes them against the current conversation history and the
// aggregated check-in digest, and returns the bounded set to inject.
//
// Retrieval is an ENHANCEMENT, not a guardrail: on any Meili error or timeout
// it logs, increments the error metric, and returns nil so coaching proceeds
// exactly as it would without long-term memory.
func (l *StreamPersonalizedCoachingLogic) retrieveMemories(
	userID, userMessage string,
	history []*aicoach.HistoryMessage,
	recentCheckInsSummary string,
) []prompts.MemorySnippet {
	if l.svcCtx.MemoryRetriever == nil || !l.svcCtx.Config.CoachMemory.Enabled {
		return nil
	}
	if userID == "" || userMessage == "" {
		return nil
	}

	timeout := l.svcCtx.Config.CoachMemory.Timeout
	if timeout <= 0 {
		timeout = defaultRetrievalTimeout
	}

	start := time.Now()
	ctx, cancel := context.WithTimeout(l.ctx, timeout)
	hits, err := l.svcCtx.MemoryRetriever.Retrieve(ctx, userID, userMessage)
	cancel()
	coachingMemoryRetrievalLatency.Observe(time.Since(start).Seconds())

	if err != nil {
		l.Errorf("coaching memory retrieval failed, proceeding without retrieval: user=%s err=%v", userID, err)
		coachingMemoryRetrievalErrors.Inc()
		return nil
	}

	coachingMemoryHits.Observe(float64(len(hits)))

	// De-dupe against what's already in the prompt: the current conversation
	// history and the aggregated check-in digest. Skip snippets that duplicate
	// either, so we only inject genuinely new long-term context.
	keep := make([]prompts.MemorySnippet, 0, len(hits))
	for _, m := range hits {
		if m.Content == "" {
			continue
		}
		if duplicatesExisting(m.Content, history, recentCheckInsSummary) {
			continue
		}
		keep = append(keep, m)
	}

	coachingMemoryHitsAfterDedupe.Observe(float64(len(keep)))
	return keep
}

// duplicatesExisting reports whether a snippet's content already appears in the
// current conversation history or the check-in digest, so we don't inject
// redundant context. Comparison is on normalized text (lowercased, whitespace
// collapsed) with a contains check; snippets are short, so this is cheap and
// good enough to avoid obvious repeats.
func duplicatesExisting(content string, history []*aicoach.HistoryMessage, recentCheckInsSummary string) bool {
	needle := normalize(content)
	if needle == "" {
		return false
	}
	if recentCheckInsSummary != "" {
		if strings.Contains(normalize(recentCheckInsSummary), needle) {
			return true
		}
	}
	for _, h := range history {
		if h == nil {
			continue
		}
		if strings.Contains(normalize(h.Content), needle) {
			return true
		}
	}
	return false
}

func normalize(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	// collapse whitespace runs to a single space
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}
