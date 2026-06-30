package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/meilisearch/meilisearch-go"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/prompts"
)

// fakeSearcher records the last request and returns a canned response.
type fakeSearcher struct {
	lastReq   *meilisearch.SearchRequest
	lastQuery string
	resp      *meilisearch.SearchResponse
	err       error
}

func (f *fakeSearcher) Search(query string, req *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error) {
	f.lastReq = req
	f.lastQuery = query
	return f.resp, f.err
}

func TestRetrieveUserIDIsolation(t *testing.T) {
	// Meili returns a hit belonging to ANOTHER user. Even though the filter
	// should prevent this, the retriever must drop it as defense-in-depth.
	crossUserHit := map[string]any{
		"id":          "check_in:other",
		"user_id":     "other-user-uuid",
		"entity_type": "check_in",
		"content":     "someone else's private note",
		"created_at":  float64(1700000000),
	}
	ownHit := map[string]any{
		"id":          "conversation_message:own",
		"user_id":     "caller-uuid",
		"entity_type": "conversation_message",
		"content":     "my past message",
		"created_at":  float64(1700000001),
		"role":        "user",
	}
	idx := &fakeSearcher{resp: &meilisearch.SearchResponse{
		Hits: []interface{}{crossUserHit, ownHit},
	}}
	r := NewRetriever(idx, Config{Limit: 5})

	got, err := r.Retrieve(context.Background(), "caller-uuid", "help with sleep")
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 isolated snippet, got %d: %+v", len(got), got)
	}
	if got[0].Content != "my past message" {
		t.Errorf("isolated snippet content = %q", got[0].Content)
	}
	if got[0].EntityType != "conversation_message" || got[0].Role != "user" {
		t.Errorf("snippet metadata wrong: %+v", got[0])
	}

	// Verify the filter actually contained the caller's user_id and the
	// entity_type whitelist.
	filter, _ := idx.lastReq.Filter.(string)
	if !contains(filter, "caller-uuid") {
		t.Errorf("filter missing caller user_id: %q", filter)
	}
	for _, et := range []string{"check_in", "conversation_message", "weekly_review"} {
		if !contains(filter, et) {
			t.Errorf("filter missing entity_type %q: %s", et, filter)
		}
	}
	if idx.lastReq.Hybrid == nil {
		t.Fatal("hybrid search not set")
	}
	if idx.lastReq.Hybrid.Embedder == "" {
		t.Error("hybrid embedder name not set")
	}
	if idx.lastReq.ShowRankingScore != true {
		t.Error("ShowRankingScore should be true for score floor + observability")
	}
}

func TestRetrieveEmptyInputs(t *testing.T) {
	r := NewRetriever(&fakeSearcher{}, Config{})
	if got, err := r.Retrieve(context.Background(), "", "q"); err != nil || got != nil {
		t.Errorf("empty userID should return nil/nil, got %v %v", got, err)
	}
	if got, err := r.Retrieve(context.Background(), "u", ""); err != nil || got != nil {
		t.Errorf("empty query should return nil/nil, got %v %v", got, err)
	}
}

func TestRetrieveFailOpenOnError(t *testing.T) {
	idx := &fakeSearcher{err: errors.New("meili down")}
	r := NewRetriever(idx, Config{})
	got, err := r.Retrieve(context.Background(), "u", "q")
	if err == nil {
		t.Fatal("expected error to propagate to caller (logic handles fail-open)")
	}
	if got != nil {
		t.Errorf("expected nil snippets on error, got %+v", got)
	}
}

func TestNewRetrieverNilIndex(t *testing.T) {
	if r := NewRetriever(nil, Config{}); r != nil {
		t.Errorf("NewRetriever(nil) should return nil")
	}
}

func TestNewRetrieverDefaults(t *testing.T) {
	r := NewRetriever(&fakeSearcher{}, Config{})
	if r == nil {
		t.Fatal("expected non-nil retriever")
	}
	if r.embedderName != "default" {
		t.Errorf("default embedder name = %q, want default", r.embedderName)
	}
	if r.semanticRatio != 0.5 {
		t.Errorf("default semantic ratio = %v, want 0.5", r.semanticRatio)
	}
	if r.limit != 5 {
		t.Errorf("default limit = %v, want 5", r.limit)
	}
	if r.scoreFloor != 0.2 {
		t.Errorf("default score floor = %v, want 0.2", r.scoreFloor)
	}
}

func TestRetrieveParsesCreatedAtAndHabitName(t *testing.T) {
	hit := map[string]any{
		"user_id":     "u1",
		"entity_type": "check_in",
		"content":     "note",
		"created_at":  float64(1700000000),
		"habit_name":  "Meditation",
	}
	idx := &fakeSearcher{resp: &meilisearch.SearchResponse{Hits: []interface{}{hit}}}
	r := NewRetriever(idx, Config{})
	got, err := r.Retrieve(context.Background(), "u1", "q")
	if err != nil || len(got) != 1 {
		t.Fatalf("Retrieve: %v len=%d", err, len(got))
	}
	if got[0].HabitName != "Meditation" {
		t.Errorf("habit name = %q", got[0].HabitName)
	}
	if got[0].CreatedAt.IsZero() {
		t.Error("created_at not parsed")
	}
	// unused import guard
	_ = prompts.MemorySnippet{}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && indexOf(s, sub) >= 0
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
