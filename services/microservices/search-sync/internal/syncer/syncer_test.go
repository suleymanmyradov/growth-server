package syncer

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/repository"
)

// fakeIndexer records which index each call hit so routing can be asserted
// without a live Meili server.
type fakeIndexer struct {
	publicUpserts  []map[string]any
	publicDeletes  []string
	memoryUpserts  []map[string]any
	memoryDeletes  []string
	hasMemory      bool
	upsertErr      error
	deleteErr      error
	memoryUpsertErr error
	memoryDeleteErr error
}

func (f *fakeIndexer) Upsert(_ context.Context, doc map[string]any) error {
	f.publicUpserts = append(f.publicUpserts, doc)
	return f.upsertErr
}
func (f *fakeIndexer) Delete(_ context.Context, docID string) error {
	f.publicDeletes = append(f.publicDeletes, docID)
	return f.deleteErr
}
func (f *fakeIndexer) UpsertMemory(_ context.Context, doc map[string]any) error {
	f.memoryUpserts = append(f.memoryUpserts, doc)
	return f.memoryUpsertErr
}
func (f *fakeIndexer) DeleteMemory(_ context.Context, docID string) error {
	f.memoryDeletes = append(f.memoryDeletes, docID)
	return f.memoryDeleteErr
}
func (f *fakeIndexer) HasMemoryIndex() bool { return f.hasMemory }

func TestIsMemoryEntity(t *testing.T) {
	cases := map[string]bool{
		"check_in":              true,
		"conversation_message":  true,
		"weekly_review":         true,
		"article":               false,
		"goal":                  false,
		"habit":                 false,
		"unknown":               false,
	}
	for et, want := range cases {
		if got := isMemoryEntity(et); got != want {
			t.Errorf("isMemoryEntity(%q) = %v, want %v", et, got, want)
		}
	}
}

func newTestSyncer(idx Indexer) *Syncer {
	return &Syncer{indexer: idx}
}

func TestUpsertDocRouting(t *testing.T) {
	idx := &fakeIndexer{hasMemory: true}
	s := newTestSyncer(idx)

	memRow := repository.OutboxRow{EntityType: "check_in", EntityID: uuid.New()}
	pubRow := repository.OutboxRow{EntityType: "article", EntityID: uuid.New()}

	if err := s.upsertDoc(context.Background(), memRow, map[string]any{"id": "check_in:x"}); err != nil {
		t.Fatalf("upsert memory: %v", err)
	}
	if err := s.upsertDoc(context.Background(), pubRow, map[string]any{"id": "article:y"}); err != nil {
		t.Fatalf("upsert public: %v", err)
	}

	if len(idx.memoryUpserts) != 1 || idx.memoryUpserts[0]["id"] != "check_in:x" {
		t.Errorf("memory upsert routed wrong: %+v", idx.memoryUpserts)
	}
	if len(idx.publicUpserts) != 1 || idx.publicUpserts[0]["id"] != "article:y" {
		t.Errorf("public upsert routed wrong: %+v", idx.publicUpserts)
	}
	if len(idx.publicDeletes) != 0 || len(idx.memoryDeletes) != 0 {
		t.Errorf("no deletes expected")
	}
}

func TestDeleteDocRouting(t *testing.T) {
	idx := &fakeIndexer{hasMemory: true}
	s := newTestSyncer(idx)

	memRow := repository.OutboxRow{EntityType: "conversation_message", EntityID: uuid.New()}
	pubRow := repository.OutboxRow{EntityType: "goal", EntityID: uuid.New()}

	if err := s.deleteDoc(context.Background(), memRow); err != nil {
		t.Fatalf("delete memory: %v", err)
	}
	if err := s.deleteDoc(context.Background(), pubRow); err != nil {
		t.Fatalf("delete public: %v", err)
	}

	if len(idx.memoryDeletes) != 1 || idx.memoryDeletes[0] != "conversation_message_"+memRow.EntityID.String() {
		t.Errorf("memory delete routed wrong: %+v", idx.memoryDeletes)
	}
	if len(idx.publicDeletes) != 1 || idx.publicDeletes[0] != "goal_"+pubRow.EntityID.String() {
		t.Errorf("public delete routed wrong: %+v", idx.publicDeletes)
	}
}

func TestUpsertDocPropagatesError(t *testing.T) {
	idx := &fakeIndexer{hasMemory: true, memoryUpsertErr: errors.New("boom")}
	s := newTestSyncer(idx)
	row := repository.OutboxRow{EntityType: "weekly_review", EntityID: uuid.New()}
	if err := s.upsertDoc(context.Background(), row, map[string]any{}); err == nil {
		t.Fatal("expected error from memory upsert, got nil")
	}
}
