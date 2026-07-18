package syncer

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
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
	ListDocIDs(ctx context.Context) ([]string, error)
	ListMemoryDocIDs(ctx context.Context) ([]string, error)
}

type Syncer struct {
	repo    *repository.Repository
	indexer Indexer
	config  config.Config
}

func NewSyncer(repo *repository.Repository, indexer Indexer, config config.Config) *Syncer {
	return &Syncer{
		repo:    repo,
		indexer: indexer,
		config:  config,
	}
}

// Run starts the syncer: it LISTENs for real-time notifications and runs
// periodic reconciliation to repair any drift caused by missed notifications
// (e.g. listener was down). It blocks until ctx is cancelled.
func (s *Syncer) Run(ctx context.Context) {
	// Run a full reconciliation on startup to catch any drift accumulated
	// while the service was down.
	s.ReconcileFull(ctx)

	notifyCh, err := s.repo.Listen(ctx)
	if err != nil {
		logx.Errorf("failed to start LISTEN, falling back to reconcile-only: %v", err)
		notifyCh = nil
	}

	reconcileTicker := time.NewTicker(s.config.Sync.ReconcileInterval)
	defer reconcileTicker.Stop()

	fullReconcileTicker := time.NewTicker(s.config.Sync.FullReconcileInterval)
	defer fullReconcileTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case n, ok := <-notifyCh:
			if !ok {
				// Channel closed (ctx cancelled); exit.
				return
			}
			s.processNotification(ctx, n)
		case <-reconcileTicker.C:
			s.ReconcileIncremental(ctx)
		case <-fullReconcileTicker.C:
			s.ReconcileFull(ctx)
		}
	}
}

// ---------------------------------------------------------------------------
// Notification-driven processing (real-time path)
// ---------------------------------------------------------------------------

func (s *Syncer) processNotification(ctx context.Context, n repository.Notification) {
	if n.Operation == "delete" {
		if err := s.deleteDoc(ctx, n.EntityType, n.EntityID); err != nil {
			logx.Errorf("notify delete %s:%s: %v", n.EntityType, n.EntityID, err)
		}
		return
	}

	// upsert
	doc, err := s.fetchDoc(ctx, n.EntityType, n.EntityID)
	if err != nil {
		if repository.IsNoRows(err) {
			// Row was deleted before we processed the notification; treat as delete.
			if delErr := s.deleteDoc(ctx, n.EntityType, n.EntityID); delErr != nil {
				logx.Errorf("notify delete (missing row) %s:%s: %v", n.EntityType, n.EntityID, delErr)
			}
			return
		}
		logx.Errorf("notify fetch %s:%s: %v", n.EntityType, n.EntityID, err)
		return
	}

	if err := s.upsertDoc(ctx, n.EntityType, doc); err != nil {
		logx.Errorf("notify upsert %s:%s: %v", n.EntityType, n.EntityID, err)
		return
	}
}

func (s *Syncer) fetchDoc(ctx context.Context, entityType string, id uuid.UUID) (map[string]any, error) {
	switch entityType {
	case "article":
		return s.repo.GetArticle(ctx, id)
	case "goal":
		return s.repo.GetGoal(ctx, id)
	case "habit":
		return s.repo.GetHabit(ctx, id)
	case "check_in":
		return s.repo.GetCheckIn(ctx, id)
	case "conversation_message":
		return s.repo.GetMessage(ctx, id)
	case "weekly_review":
		return s.repo.GetWeeklyReview(ctx, id)
	default:
		return nil, fmt.Errorf("unknown entity type: %s", entityType)
	}
}

// ---------------------------------------------------------------------------
// Reconciliation (correctness floor)
// ---------------------------------------------------------------------------

// ReconcileIncremental upserts all rows that changed in the last
// ReconcileInterval * 2 window. This catches any notifications that were lost
// while the LISTEN connection was briefly down. It does NOT delete orphans —
// that's the job of ReconcileFull.
func (s *Syncer) ReconcileIncremental(ctx context.Context) {
	// Use 2x the tick interval as the look-back window to overlap ticks and
	// avoid missing rows that land on a boundary.
	since := time.Now().Add(-2 * s.config.Sync.ReconcileInterval)
	start := time.Now()
	var total int

	total += s.reconcileUpserts(ctx, "article", func(ctx context.Context) ([]uuid.UUID, error) {
		return s.repo.ListRecentArticleIDs(ctx, since)
	})
	total += s.reconcileUpserts(ctx, "goal", func(ctx context.Context) ([]uuid.UUID, error) {
		return s.repo.ListRecentGoalIDs(ctx, since)
	})
	total += s.reconcileUpserts(ctx, "habit", func(ctx context.Context) ([]uuid.UUID, error) {
		return s.repo.ListRecentHabitIDs(ctx, since)
	})
	total += s.reconcileUpserts(ctx, "check_in", func(ctx context.Context) ([]uuid.UUID, error) {
		return s.repo.ListRecentCheckInIDs(ctx, since)
	})
	total += s.reconcileUpserts(ctx, "conversation_message", func(ctx context.Context) ([]uuid.UUID, error) {
		return s.repo.ListRecentMessageIDs(ctx, since)
	})
	total += s.reconcileUpserts(ctx, "weekly_review", func(ctx context.Context) ([]uuid.UUID, error) {
		return s.repo.ListRecentWeeklyReviewIDs(ctx, since)
	})

	logx.Infof("incremental reconcile: %d rows upserted in %s", total, time.Since(start).Round(time.Millisecond))
}

// ReconcileFull compares every Postgres row against Meilisearch: upserts
// missing docs and deletes orphaned docs that no longer have a source row.
// Run on startup and on a slow cadence (e.g. daily).
func (s *Syncer) ReconcileFull(ctx context.Context) {
	start := time.Now()
	var upserts, deletes int

	// Public catalog entities
	upserts += s.reconcileUpserts(ctx, "article", s.repo.ListArticleIDs)
	upserts += s.reconcileUpserts(ctx, "goal", s.repo.ListGoalIDs)
	upserts += s.reconcileUpserts(ctx, "habit", s.repo.ListHabitIDs)

	// Memory index entities
	upserts += s.reconcileUpserts(ctx, "check_in", s.repo.ListCheckInIDs)
	upserts += s.reconcileUpserts(ctx, "conversation_message", s.repo.ListMessageIDs)
	upserts += s.reconcileUpserts(ctx, "weekly_review", s.repo.ListWeeklyReviewIDs)

	// Orphan detection: compare Meilisearch doc IDs against the union of all
	// Postgres IDs and delete docs whose source row no longer exists.
	deletes += s.reconcileOrphans(ctx)

	logx.Infof("full reconcile: %d upserts, %d orphan deletes in %s",
		upserts, deletes, time.Since(start).Round(time.Millisecond))
}

// reconcileUpserts fetches and upserts each ID in the list. Returns the number
// of successful upserts.
func (s *Syncer) reconcileUpserts(ctx context.Context, entityType string, listIDs func(ctx context.Context) ([]uuid.UUID, error)) int {
	ids, err := listIDs(ctx)
	if err != nil {
		logx.Errorf("reconcile list %s: %v", entityType, err)
		return 0
	}

	var count int
	for _, id := range ids {
		if ctx.Err() != nil {
			return count
		}
		doc, err := s.fetchDoc(ctx, entityType, id)
		if err != nil {
			logx.Errorf("reconcile fetch %s:%s: %v", entityType, id, err)
			continue
		}
		if err := s.upsertDoc(ctx, entityType, doc); err != nil {
			logx.Errorf("reconcile upsert %s:%s: %v", entityType, id, err)
			continue
		}
		count++
	}
	return count
}

// reconcileOrphans lists all Meilisearch doc IDs, builds the set of expected
// IDs from Postgres, and deletes any Meili doc whose ID is not in the expected
// set. Returns the number of deletes.
func (s *Syncer) reconcileOrphans(ctx context.Context) int {
	// Build the set of expected doc IDs from Postgres.
	expected := make(map[string]struct{})

	publicIDs := [][]uuid.UUID{
		mustList(ctx, s.repo.ListArticleIDs),
		mustList(ctx, s.repo.ListGoalIDs),
		mustList(ctx, s.repo.ListHabitIDs),
	}
	memoryIDs := [][]uuid.UUID{
		mustList(ctx, s.repo.ListCheckInIDs),
		mustList(ctx, s.repo.ListMessageIDs),
		mustList(ctx, s.repo.ListWeeklyReviewIDs),
	}

	// Map entity type prefixes to their ID lists for correct docID construction.
	publicSpecs := []struct {
		et   string
		ids  []uuid.UUID
	}{
		{"article", publicIDs[0]},
		{"goal", publicIDs[1]},
		{"habit", publicIDs[2]},
	}
	memorySpecs := []struct {
		et   string
		ids  []uuid.UUID
	}{
		{"check_in", memoryIDs[0]},
		{"conversation_message", memoryIDs[1]},
		{"weekly_review", memoryIDs[2]},
	}

	for _, spec := range publicSpecs {
		for _, id := range spec.ids {
			expected[repository.DocID(spec.et, id)] = struct{}{}
		}
	}
	for _, spec := range memorySpecs {
		for _, id := range spec.ids {
			expected[repository.DocID(spec.et, id)] = struct{}{}
		}
	}

	var deletes int

	// Public catalog orphans
	deletes += s.deleteOrphansFromIndex(ctx, expected, false)

	// Memory index orphans (only if memory index is configured)
	if s.indexer.HasMemoryIndex() {
		// For the memory index, the expected set is only the memory entities.
		memoryExpected := make(map[string]struct{})
		for _, spec := range memorySpecs {
			for _, id := range spec.ids {
				memoryExpected[repository.DocID(spec.et, id)] = struct{}{}
			}
		}
		deletes += s.deleteOrphansFromIndex(ctx, memoryExpected, true)
	}

	return deletes
}

func (s *Syncer) deleteOrphansFromIndex(ctx context.Context, expected map[string]struct{}, memory bool) int {
	var docIDs []string
	var err error
	if memory {
		docIDs, err = s.indexer.ListMemoryDocIDs(ctx)
	} else {
		docIDs, err = s.indexer.ListDocIDs(ctx)
	}
	if err != nil {
		logx.Errorf("reconcile list meili docs (memory=%v): %v", memory, err)
		return 0
	}

	var count int
	for _, docID := range docIDs {
		if ctx.Err() != nil {
			return count
		}
		if _, ok := expected[docID]; ok {
			continue
		}
		// Orphan: delete from the correct index.
		var delErr error
		if memory {
			delErr = s.indexer.DeleteMemory(ctx, docID)
		} else {
			delErr = s.indexer.Delete(ctx, docID)
		}
		if delErr != nil {
			logx.Errorf("reconcile delete orphan %s (memory=%v): %v", docID, memory, delErr)
			continue
		}
		count++
	}
	return count
}

func mustList(ctx context.Context, f func(ctx context.Context) ([]uuid.UUID, error)) []uuid.UUID {
	ids, err := f(ctx)
	if err != nil {
		logx.Errorf("reconcile list ids: %v", err)
		return nil
	}
	return ids
}

// ---------------------------------------------------------------------------
// Routing helpers (shared by notification + reconcile paths)
// ---------------------------------------------------------------------------

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
func (s *Syncer) upsertDoc(ctx context.Context, entityType string, doc map[string]any) error {
	if isMemoryEntity(entityType) {
		return s.indexer.UpsertMemory(ctx, doc)
	}
	return s.indexer.Upsert(ctx, doc)
}

// deleteDoc routes a delete to the correct index. The doc id scheme
// ("<entity_type>_<uuid>") is shared across both indexes.
func (s *Syncer) deleteDoc(ctx context.Context, entityType string, id uuid.UUID) error {
	docID := repository.DocID(entityType, id)
	if isMemoryEntity(entityType) {
		return s.indexer.DeleteMemory(ctx, docID)
	}
	return s.indexer.Delete(ctx, docID)
}
