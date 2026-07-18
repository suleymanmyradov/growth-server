package indexer

import (
	"context"
	"fmt"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

type MeiliIndexer struct {
	index       meilisearch.IndexManager
	memoryIndex meilisearch.IndexManager
}

// NewMeiliIndexer wraps the public catalog index. memoryIndex is the optional
// private user_memory index; pass nil when the memory index is not configured.
func NewMeiliIndexer(index, memoryIndex meilisearch.IndexManager) *MeiliIndexer {
	return &MeiliIndexer{index: index, memoryIndex: memoryIndex}
}

// taskWaitInterval is the polling interval used when waiting for a Meili task
// to complete. It is deliberately larger than the public-catalog 100ms because
// the user_memory index embeds documents via an external embedder (Ollama),
// and each upsert task can take several seconds. The public catalog has no
// embedder, so its tasks complete in milliseconds.
const taskWaitInterval = 500 * time.Millisecond

func (i *MeiliIndexer) Upsert(ctx context.Context, doc map[string]any) error {
	task, err := i.index.AddDocuments(doc)
	if err != nil {
		return fmt.Errorf("meilisearch upsert: %w", err)
	}
	_, err = i.index.WaitForTaskWithContext(ctx, task.TaskUID, taskWaitInterval)
	if err != nil {
		return fmt.Errorf("meilisearch upsert wait: %w", err)
	}
	return nil
}

func (i *MeiliIndexer) Delete(ctx context.Context, docID string) error {
	task, err := i.index.DeleteDocument(docID)
	if err != nil {
		return fmt.Errorf("meilisearch delete: %w", err)
	}
	_, err = i.index.WaitForTaskWithContext(ctx, task.TaskUID, taskWaitInterval)
	if err != nil {
		return fmt.Errorf("meilisearch delete wait: %w", err)
	}
	return nil
}

// UpsertMemory writes a doc to the private user_memory index. It is a no-op
// (returns nil) when the memory index is not configured, so the syncer can
// call it unconditionally and degrade gracefully.
func (i *MeiliIndexer) UpsertMemory(ctx context.Context, doc map[string]any) error {
	if i.memoryIndex == nil {
		return nil
	}
	task, err := i.memoryIndex.AddDocuments(doc)
	if err != nil {
		return fmt.Errorf("meilisearch memory upsert: %w", err)
	}
	_, err = i.memoryIndex.WaitForTaskWithContext(ctx, task.TaskUID, taskWaitInterval)
	if err != nil {
		return fmt.Errorf("meilisearch memory upsert wait: %w", err)
	}
	return nil
}

// DeleteMemory removes a doc from the private user_memory index. No-op when
// the memory index is not configured.
func (i *MeiliIndexer) DeleteMemory(ctx context.Context, docID string) error {
	if i.memoryIndex == nil {
		return nil
	}
	task, err := i.memoryIndex.DeleteDocument(docID)
	if err != nil {
		return fmt.Errorf("meilisearch memory delete: %w", err)
	}
	_, err = i.memoryIndex.WaitForTaskWithContext(ctx, task.TaskUID, taskWaitInterval)
	if err != nil {
		return fmt.Errorf("meilisearch memory delete wait: %w", err)
	}
	return nil
}

// HasMemoryIndex reports whether the private memory index is configured.
func (i *MeiliIndexer) HasMemoryIndex() bool {
	return i.memoryIndex != nil
}

// ListDocIDs returns all document primary-key IDs from the public catalog
// index, paginating through the full set. Used by the reconciliation loop to
// detect orphaned docs that no longer have a corresponding Postgres row.
func (i *MeiliIndexer) ListDocIDs(ctx context.Context) ([]string, error) {
	return listDocIDs(ctx, i.index)
}

// ListMemoryDocIDs returns all document IDs from the private memory index.
// Returns nil (no error) when the memory index is not configured.
func (i *MeiliIndexer) ListMemoryDocIDs(ctx context.Context) ([]string, error) {
	if i.memoryIndex == nil {
		return nil, nil
	}
	return listDocIDs(ctx, i.memoryIndex)
}

const docIDPageSize = 1000

func listDocIDs(ctx context.Context, idx meilisearch.IndexManager) ([]string, error) {
	var allIDs []string
	var offset int64
	for {
		var resp meilisearch.DocumentsResult
		if err := idx.GetDocumentsWithContext(ctx, &meilisearch.DocumentsQuery{
			Offset: offset,
			Limit:  docIDPageSize,
			Fields: []string{"id"},
		}, &resp); err != nil {
			return nil, fmt.Errorf("list docs (offset %d): %w", offset, err)
		}
		for _, doc := range resp.Results {
			if id, ok := doc["id"].(string); ok {
				allIDs = append(allIDs, id)
			}
		}
		if int64(len(resp.Results)) < docIDPageSize {
			break
		}
		offset += docIDPageSize
	}
	return allIDs, nil
}
