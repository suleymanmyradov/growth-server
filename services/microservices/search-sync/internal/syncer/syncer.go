package syncer

import (
	"context"
	"fmt"
	"time"

	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/config"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/indexer"
	"github.com/suleymanmyradov/growth-server/services/microservices/search-sync/internal/repository"
	"github.com/zeromicro/go-zero/core/logx"
)

type Syncer struct {
	repo    *repository.OutboxRepository
	indexer *indexer.MeiliIndexer
	config  config.Config
}

func NewSyncer(repo *repository.OutboxRepository, indexer *indexer.MeiliIndexer, config config.Config) *Syncer {
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
	default:
		err = fmt.Errorf("unknown entity type: %s", row.EntityType)
	}

	if err != nil {
		if repository.IsNoRows(err) {
			// Entity was deleted before sync ran; treat as delete
			if delErr := s.indexer.Delete(ctx, fmt.Sprintf("%s:%s", row.EntityType, row.EntityID.String())); delErr != nil {
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
		if delErr := s.indexer.Delete(ctx, fmt.Sprintf("%s:%s", row.EntityType, row.EntityID.String())); delErr != nil {
			logx.Errorf("delete %s:%s: %v", row.EntityType, row.EntityID, delErr)
			s.retryOrFail(ctx, row, delErr)
			return
		}
	} else {
		if upsertErr := s.indexer.Upsert(ctx, doc); upsertErr != nil {
			logx.Errorf("upsert %s:%s: %v", row.EntityType, row.EntityID, upsertErr)
			s.retryOrFail(ctx, row, upsertErr)
			return
		}
	}

	if markErr := s.repo.MarkProcessed(ctx, row.ID); markErr != nil {
		logx.Errorf("mark processed %s:%s: %v", row.EntityType, row.EntityID, markErr)
	}
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
