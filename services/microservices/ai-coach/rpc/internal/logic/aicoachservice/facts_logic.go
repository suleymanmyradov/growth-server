package aicoachservicelogic

import (
	"context"
	"time"

	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/facts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/prompts"
)

// defaultFactExtractionTimeout bounds the post-turn extraction call. Extraction
// runs after the user already has their answer, so it must never be able to
// delay or fail the turn it follows.
const defaultFactExtractionTimeout = 5 * time.Second

// loadFacts returns the curated facts to inject for this turn.
//
// Fail-open, for the same reason retrieval is: coaching without curated memory
// is a less personal answer, not a broken one. A database hiccup must not cost
// the user their turn.
func (l *StreamPersonalizedCoachingLogic) loadFacts(userID string) []prompts.UserFact {
	if l.svcCtx.FactStore == nil || !l.svcCtx.Config.CoachMemory.FactsEnabled {
		return nil
	}
	if userID == "" {
		return nil
	}

	known, err := l.svcCtx.FactStore.ForPrompt(l.ctx, userID)
	if err != nil {
		l.Errorf("curated memory load failed, proceeding without it: user=%s err=%v", userID, err)
		return nil
	}
	return known
}

// extractFacts runs write-time curation on a completed exchange and persists
// what survives validation.
//
// Called after the terminal stream chunk has been sent, so every failure path
// here is a log line and nothing more — the user's turn is already complete and
// durable by this point. Extraction is best-effort by design: a fact missed
// this turn can be picked up on the next one, whereas a turn lost to a failed
// extraction is not recoverable.
//
// Takes an explicit ctx rather than using l.ctx because the request context may
// already be cancelled once the stream closes.
func (l *StreamPersonalizedCoachingLogic) extractFacts(ctx context.Context, userID, userMessage, assistantMessage string) {
	if l.svcCtx.FactStore == nil || l.svcCtx.FactExtractor == nil {
		return
	}
	if !l.svcCtx.Config.CoachMemory.FactExtractionEnabled {
		return
	}
	if userID == "" || userMessage == "" || assistantMessage == "" {
		return
	}

	timeout := l.svcCtx.Config.CoachMemory.FactExtractionTimeout
	if timeout <= 0 {
		timeout = defaultFactExtractionTimeout
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Show the extractor what is already believed so it can flag a
	// contradiction as a supersession instead of silently storing both versions.
	existing := make([]facts.ExistingFact, 0, 8)
	if current, err := l.svcCtx.FactStore.ForPrompt(ctx, userID); err == nil {
		for _, f := range current {
			existing = append(existing, facts.ExistingFact{Fact: f.Fact})
		}
	}

	candidates, err := l.svcCtx.FactExtractor.Extract(ctx, userMessage, assistantMessage, "", existing)
	if err != nil {
		l.Errorf("fact extraction failed: user=%s err=%v", userID, err)
		factExtractionErrors.Inc()
		return
	}

	stored := 0
	for _, c := range candidates {
		if _, ok, err := l.svcCtx.FactStore.Record(ctx, userID, c); err != nil {
			l.Errorf("fact record failed: user=%s err=%v", userID, err)
			factExtractionErrors.Inc()
		} else if ok {
			stored++
		}
	}
	factsStored.Add(float64(stored))
}
