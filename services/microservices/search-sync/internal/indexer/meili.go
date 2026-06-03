package indexer

import (
	"context"
	"fmt"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

type MeiliIndexer struct {
	index meilisearch.IndexManager
}

func NewMeiliIndexer(index meilisearch.IndexManager) *MeiliIndexer {
	return &MeiliIndexer{index: index}
}

func (i *MeiliIndexer) Upsert(ctx context.Context, doc map[string]any) error {
	task, err := i.index.AddDocuments(doc)
	if err != nil {
		return fmt.Errorf("meilisearch upsert: %w", err)
	}
	_, err = i.index.WaitForTaskWithContext(ctx, task.TaskUID, 100*time.Millisecond)
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
	_, err = i.index.WaitForTaskWithContext(ctx, task.TaskUID, 100*time.Millisecond)
	if err != nil {
		return fmt.Errorf("meilisearch delete wait: %w", err)
	}
	return nil
}
