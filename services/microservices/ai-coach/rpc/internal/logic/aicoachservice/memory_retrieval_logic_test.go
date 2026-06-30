package aicoachservicelogic

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/meilisearch/meilisearch-go"
	dto "github.com/prometheus/client_model/go"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/memory"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"
	"github.com/zeromicro/go-zero/core/logx"
)

// counterValue reads a prometheus counter's current value via the dto package
// (already an indirect dep) so we can assert fail-open metrics without pulling
// in prometheus/testutil (which adds a new transitive dependency).
func counterValue(c interface{ Write(*dto.Metric) error }) float64 {
	m := &dto.Metric{}
	_ = c.Write(m)
	return m.GetCounter().GetValue()
}

// errSearcher satisfies memory's unexported searcher interface and always
// errors, simulating Meili being down.
type errSearcher struct{}

func (errSearcher) Search(string, *meilisearch.SearchRequest) (*meilisearch.SearchResponse, error) {
	return nil, errors.New("meili down")
}

func newTestLogic(retriever *memory.Retriever, enabled bool) *StreamPersonalizedCoachingLogic {
	return &StreamPersonalizedCoachingLogic{
		ctx: context.Background(),
		svcCtx: &svc.ServiceContext{
			Config: config.Config{
				CoachMemory: config.CoachMemoryConfig{Enabled: enabled, Timeout: 100 * time.Millisecond},
			},
			MemoryRetriever: retriever,
		},
		Logger: logx.WithContext(context.Background()),
	}
}

// TestRetrieveMemories_FailOpenOnMeiliError verifies that a Meili error yields
// no snippets (coaching proceeds without retrieval) and increments the error
// metric so the outage is observable.
func TestRetrieveMemories_FailOpenOnMeiliError(t *testing.T) {
	l := newTestLogic(memory.NewRetriever(errSearcher{}, memory.Config{}), true)

	before := counterValue(coachingMemoryRetrievalErrors)
	got := l.retrieveMemories("user1", "help with sleep", nil, "")
	after := counterValue(coachingMemoryRetrievalErrors)

	if got != nil {
		t.Errorf("expected nil snippets on Meili error (fail-open), got %+v", got)
	}
	if after <= before {
		t.Errorf("expected coachingMemoryRetrievalErrors to increment: before=%v after=%v", before, after)
	}
}

// TestRetrieveMemories_DisabledFlagReturnsNil verifies the feature flag gates
// retrieval even when a retriever is configured.
func TestRetrieveMemories_DisabledFlagReturnsNil(t *testing.T) {
	l := newTestLogic(memory.NewRetriever(errSearcher{}, memory.Config{}), false)
	if got := l.retrieveMemories("user1", "help", nil, ""); got != nil {
		t.Errorf("disabled flag should return nil, got %+v", got)
	}
}

// TestRetrieveMemories_NilRetrieverReturnsNil verifies new users / unconfigured
// deployments behave exactly as today.
func TestRetrieveMemories_NilRetrieverReturnsNil(t *testing.T) {
	l := newTestLogic(nil, true)
	if got := l.retrieveMemories("user1", "help", nil, ""); got != nil {
		t.Errorf("nil retriever should return nil, got %+v", got)
	}
}

// TestDuplicatesExisting verifies snippets already present in the conversation
// history or check-in digest are de-duped.
func TestDuplicatesExisting(t *testing.T) {
	history := []*aicoach.HistoryMessage{
		{Role: "user", Content: "I struggled with sleep last week"},
	}
	if !duplicatesExisting("I struggled with sleep", history, "") {
		t.Error("expected snippet duplicating history to be detected")
	}
	if !duplicatesExisting("focused", nil, "Recent activity: stayed focused") {
		t.Error("expected snippet duplicating check-in digest to be detected")
	}
	if duplicatesExisting("a brand new unique note", history, "Recent activity: 30 check-ins") {
		t.Error("unique snippet should not be de-duped")
	}
}
