package syncer

import (
	"context"
	"fmt"
	"time"

	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/repository"
	"github.com/zeromicro/go-zero/core/logx"
)

// Indexer is the subset of the Meili indexer the syncer needs. Defining it
// here lets routing be unit-tested with a fake instead of a live Meili server.
// *indexer.MeiliIndexer satisfies this interface.
type Indexer interface {
	Upsert(ctx context.Context, doc map[string]any) error
	Delete(ctx context.Context, docID string) error
	UpsertMemory(ctx context.Context, doc map[string]any) error
	DeleteMemory(ctx context.Context, docID string) error
	HasMemoryIndex() bool
}

type Syncer struct {
	repo    *repository.OutboxRepository
	indexer Indexer
	config  config.Config
}

func NewSyncer(repo *repository.OutboxRepository, indexer Indexer, config config.Config) *Syncer {
	return &Syncer{
		repo:    repo,
		indexer: indexer,
		config:  config,
	}
}

func (s *Syncer) Run(ctx context.Context) {
	// Start LISTEN/NOTIFY loop for low-latency wakeups
	notifyCh, err := s.repo.ListenNotify(ctx)
	if err != nil {
		logx.Errorf("failed to start LISTEN/NOTIFY: %v", err)
		// Fall back to polling only
		notifyCh = make(chan struct{})
	}

	// Start stale lock reaper
	ticker := time.NewTicker(s.config.Sync.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.processBatch(ctx)
		case <-notifyCh:
			s.processBatch(ctx)
		}
	}
}

func (s *Syncer) processBatch(ctx context.Context) {
	if err := s.repo.ReleaseStaleLocks(ctx, s.config.Sync.LockTimeout); err != nil {
		logx.Errorf("release stale locks: %v", err)
	}

	rows, err := s.repo.LockPending(ctx, s.config.Sync.BatchSize, s.config.Sync.WorkerID, s.config.Sync.LockTimeout)
	if err != nil {
		logx.Errorf("lock pending: %v", err)
		return
	}

	for _, row := range rows {
		s.processRow(ctx, row)
	}
}

func (s *Syncer) processRow(ctx context.Context, row repository.OutboxRow) {
	var doc map[string]any
	var err error

	switch row.EntityType {
	case "article":
		doc, err = s.repo.GetArticle(ctx, row.EntityID)
	case "goal":
		doc, err = s.repo.GetGoal(ctx, row.EntityID)
	case "habit":
		doc, err = s.repo.GetHabit(ctx, row.EntityID)
	case "check_in":
		doc, err = s.repo.GetCheckIn(ctx, row.EntityID)
	case "conversation_message":
		doc, err = s.repo.GetMessage(ctx, row.EntityID)
	case "weekly_review":
		doc, err = s.repo.GetWeeklyReview(ctx, row.EntityID)
	default:
		err = fmt.Errorf("unknown entity type: %s", row.EntityType)
	}

	if err != nil {
		if repository.IsNoRows(err) {
			// Entity was deleted before sync ran; treat as delete.
			if delErr := s.deleteDoc(ctx, row); delErr != nil {
				logx.Errorf("delete missing %s:%s: %v", row.EntityType, row.EntityID, delErr)
				s.retryOrFail(ctx, row, delErr)
				return
			}
			if markErr := s.repo.MarkProcessed(ctx, row.ID); markErr != nil {
				logx.Errorf("mark processed after delete %s:%s: %v", row.EntityType, row.EntityID, markErr)
			}
			return
		}
		logx.Errorf("fetch %s:%s: %v", row.EntityType, row.EntityID, err)
		s.retryOrFail(ctx, row, err)
		return
	}

	if row.Operation == "delete" {
		if delErr := s.deleteDoc(ctx, row); delErr != nil {
			logx.Errorf("delete %s:%s: %v", row.EntityType, row.EntityID, delErr)
			s.retryOrFail(ctx, row, delErr)
			return
		}
	} else {
		if upsertErr := s.upsertDoc(ctx, row, doc); upsertErr != nil {
			logx.Errorf("upsert %s:%s: %v", row.EntityType, row.EntityID, upsertErr)
			s.retryOrFail(ctx, row, upsertErr)
			return
		}
	}

	if markErr := s.repo.MarkProcessed(ctx, row.ID); markErr != nil {
		logx.Errorf("mark processed %s:%s: %v", row.EntityType, row.EntityID, markErr)
	}
}

// isMemoryEntity reports whether an entity type belongs to the private
// user_memory index (per-user free-text) rather than the public catalog.
func isMemoryEntity(entityType string) bool {
	switch entityType {
	case "check_in", "conversation_message", "weekly_review":
		return true
	default:
		return false
	}
}

// upsertDoc routes an upsert to the correct index: the private user_memory
// index for memory entity types, the public catalog index otherwise.
func (s *Syncer) upsertDoc(ctx context.Context, row repository.OutboxRow, doc map[string]any) error {
	if isMemoryEntity(row.EntityType) {
		return s.indexer.UpsertMemory(ctx, doc)
	}
	return s.indexer.Upsert(ctx, doc)
}

// deleteDoc routes a delete to the correct index. The doc id scheme
// ("<entity_type>:<uuid>") is shared across both indexes.
func (s *Syncer) deleteDoc(ctx context.Context, row repository.OutboxRow) error {
	docID := fmt.Sprintf("%s_%s", row.EntityType, row.EntityID.String())
	if isMemoryEntity(row.EntityType) {
		return s.indexer.DeleteMemory(ctx, docID)
	}
	return s.indexer.Delete(ctx, docID)
}

func (s *Syncer) retryOrFail(ctx context.Context, row repository.OutboxRow, err error) {
	availableAt := s.nextRetryTime(row.Attempts)
	if row.Attempts >= s.config.Sync.MaxAttempts {
		logx.Errorf("max attempts reached for %s:%s, marking failed", row.EntityType, row.EntityID)
	}
	if markErr := s.repo.MarkFailed(ctx, row.ID, err.Error(), availableAt); markErr != nil {
		logx.Errorf("mark failed %s:%s: %v", row.EntityType, row.EntityID, markErr)
	}
}

func (s *Syncer) nextRetryTime(attempts int) time.Time {
	var delay time.Duration
	switch attempts {
	case 1:
		delay = 5 * time.Second
	case 2:
		delay = 30 * time.Second
	case 3:
		delay = 2 * time.Minute
	case 4:
		delay = 10 * time.Minute
	default:
		delay = 30 * time.Minute
	}
	return time.Now().Add(delay)
}

func (s *Syncer) Backfill(ctx context.Context) error {
	return s.repo.Backfill(ctx)
}
