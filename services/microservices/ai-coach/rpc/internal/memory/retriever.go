// Package memory retrieves long-term, cross-conversation memory snippets for
// the AI coach from the private `user_memory` Meili index.
//
// Privacy contract: every query is hybrid-searched with a hard filter
// `user_id = <caller> AND entity_type IN (...)`. As defense-in-depth, each
// returned hit is re-checked against the caller's user_id before being
// returned, so a misconfigured filter can never leak another user's memories.
package memory

import (
	"context"
	"fmt"
	"time"

	"github.com/meilisearch/meilisearch-go"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/prompts"
)

// searcher is the minimal Meili interface the retriever needs. Defining it
// here keeps the retriever unit-testable with a fake instead of the full
// (large) meilisearch.IndexManager interface. meilisearch.IndexManager
// satisfies searcher.
type searcher interface {
	Search(query string, request *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error)
}

// Config controls retrieval behavior. All fields have safe defaults applied by
// NewRetriever.
type Config struct {
	EmbedderName  string  // hybrid embedder key; defaults to "default"
	SemanticRatio float64 // 0..1 balance between keyword and semantic; default 0.5
	Limit         int     // max hits to request from Meili; default 5
	ScoreFloor    float64 // minimum _rankingScore; default 0.2
}

// Retriever is a read-only client over the user_memory index. It is safe for
// concurrent use.
type Retriever struct {
	index         searcher
	embedderName  string
	semanticRatio float64
	limit         int64
	scoreFloor    float64
}

// NewRetriever wraps a Meili index manager with retrieval config. Returns nil
// if the index is nil (caller should treat nil as "retrieval disabled").
func NewRetriever(idx searcher, cfg Config) *Retriever {
	if idx == nil {
		return nil
	}
	if cfg.EmbedderName == "" {
		cfg.EmbedderName = "default"
	}
	if cfg.SemanticRatio <= 0 || cfg.SemanticRatio > 1 {
		cfg.SemanticRatio = 0.5
	}
	if cfg.Limit <= 0 {
		cfg.Limit = 5
	}
	if cfg.ScoreFloor <= 0 {
		cfg.ScoreFloor = 0.2
	}
	return &Retriever{
		index:         idx,
		embedderName:  cfg.EmbedderName,
		semanticRatio: cfg.SemanticRatio,
		limit:         int64(cfg.Limit),
		scoreFloor:    cfg.ScoreFloor,
	}
}

// memoryEntityTypes is the closed set of entity types retrievable from
// user_memory. It is applied in every query filter.
var memoryEntityTypes = []string{"check_in", "conversation_message", "weekly_review"}

// Retrieve runs a hybrid search over user_memory scoped to userID and returns
// the top snippets above the score floor. The caller MUST pass the caller's
// user id; the filter enforces isolation server-side and this method re-checks
// each hit client-side.
func (r *Retriever) Retrieve(ctx context.Context, userID, query string) ([]prompts.MemorySnippet, error) {
	if r == nil || r.index == nil {
		return nil, nil
	}
	if userID == "" || query == "" {
		return nil, nil
	}

	filter := fmt.Sprintf("user_id = '%s' AND entity_type IN ['check_in','conversation_message','weekly_review']", userID)
	req := &meilisearch.SearchRequest{
		Query:                 query,
		Filter:                filter,
		Limit:                 r.limit,
		Hybrid:                &meilisearch.SearchRequestHybrid{Embedder: r.embedderName, SemanticRatio: r.semanticRatio},
		ShowRankingScore:      true,
		RankingScoreThreshold: r.scoreFloor,
	}

	resp, err := r.index.Search(query, req)
	if err != nil {
		return nil, fmt.Errorf("memory search: %w", err)
	}
	if resp == nil || len(resp.Hits) == 0 {
		return nil, nil
	}

	snippets := make([]prompts.MemorySnippet, 0, len(resp.Hits))
	for _, h := range resp.Hits {
		hit, ok := h.(map[string]any)
		if !ok {
			continue
		}
		// Defense-in-depth: never return a hit whose user_id is not the
		// caller's, even if the filter should have excluded it.
		if hitUserID(hit) != userID {
			continue
		}
		snippets = append(snippets, prompts.MemorySnippet{
			EntityType: stringField(hit, "entity_type"),
			Content:    stringField(hit, "content"),
			CreatedAt:  timeField(hit, "created_at"),
			HabitName:  stringField(hit, "habit_name"),
			Role:       stringField(hit, "role"),
		})
	}
	return snippets, nil
}

func hitUserID(hit map[string]any) string {
	return stringField(hit, "user_id")
}

func stringField(hit map[string]any, key string) string {
	v, ok := hit[key]
	if !ok || v == nil {
		return ""
	}
	switch s := v.(type) {
	case string:
		return s
	default:
		return fmt.Sprintf("%v", v)
	}
}

// timeField decodes the created_at unix timestamp stored on memory docs.
func timeField(hit map[string]any, key string) time.Time {
	v, ok := hit[key]
	if !ok || v == nil {
		return time.Time{}
	}
	switch n := v.(type) {
	case float64:
		return time.Unix(int64(n), 0).UTC()
	case int64:
		return time.Unix(n, 0).UTC()
	case time.Time:
		return n
	default:
		return time.Time{}
	}
}
