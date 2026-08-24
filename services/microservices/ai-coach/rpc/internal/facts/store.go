// Package facts is the curated tier of the coach's long-term memory: durable,
// categorised statements about a user, extracted at write time.
//
// It exists because retrieval over raw conversation messages cannot answer two
// questions that matter more than relevance:
//
//   - Is this a durable fact or a passing mood? "I train Tuesdays and
//     Thursdays" and "I feel awful today" are equally retrievable and must not
//     be treated alike.
//   - Has this been replaced? "I'm quitting smoking" (March) and "I started
//     again" (August) both match a query about smoking, and a ranker has no
//     notion that the second supersedes the first.
//
// Both are write-time curation problems. No amount of ranking tuning fixes
// them, which is why this sits alongside the user_memory search index rather
// than replacing it: facts are injected into the prompt by default, and the
// index is searched on demand.
package facts

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/prompts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/repository/db"
)

// Queries is the subset of the generated query set this package needs, so the
// store can be unit-tested with a fake instead of a live database.
type Queries interface {
	ListCurrentUserFacts(ctx context.Context, userID uuid.UUID, confidence float32, limit int32) ([]db.UserFact, error)
	CreateUserFact(ctx context.Context, arg db.CreateUserFactParams) (db.UserFact, error)
	SupersedeUserFact(ctx context.Context, id uuid.UUID, userID uuid.UUID, supersededBy uuid.NullUUID) (db.UserFact, error)
	ForgetUserFact(ctx context.Context, id uuid.UUID, userID uuid.UUID) error
	ForgetAllUserFacts(ctx context.Context, userID uuid.UUID) error
}

// Config bounds what reaches the prompt. Zero values get the defaults applied
// by NewStore.
type Config struct {
	// MinConfidence is the floor for a fact to be injected into the prompt.
	// Below it, a fact is still stored and still visible to the user, but the
	// coach does not act on it. Default 0.5.
	MinConfidence float32
	// MaxFacts bounds how many facts are injected per turn. Default 20.
	MaxFacts int32
}

// Store reads and writes curated facts. Safe for concurrent use.
type Store struct {
	q             Queries
	minConfidence float32
	maxFacts      int32
}

// NewStore returns a Store, or nil if q is nil so callers can treat nil as
// "curated memory disabled" and skip it without branching everywhere.
func NewStore(q Queries, cfg Config) *Store {
	if q == nil {
		return nil
	}
	if cfg.MinConfidence <= 0 || cfg.MinConfidence > 1 {
		cfg.MinConfidence = 0.5
	}
	if cfg.MaxFacts <= 0 {
		cfg.MaxFacts = 20
	}
	return &Store{q: q, minConfidence: cfg.MinConfidence, maxFacts: cfg.MaxFacts}
}

// ForPrompt returns the facts to inject for this turn, already filtered by
// confidence and bounded in count.
//
// Errors are returned rather than swallowed, but callers on the coaching path
// should fail open: coaching without curated memory is degraded, not broken.
func (s *Store) ForPrompt(ctx context.Context, userID string) ([]prompts.UserFact, error) {
	if s == nil {
		return nil, nil
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("facts: invalid user id: %w", err)
	}

	rows, err := s.q.ListCurrentUserFacts(ctx, uid, s.minConfidence, s.maxFacts)
	if err != nil {
		return nil, fmt.Errorf("facts: list current: %w", err)
	}

	out := make([]prompts.UserFact, 0, len(rows))
	for _, r := range rows {
		// Defense-in-depth: the query filters on user_id, but a fact belonging
		// to someone else must never reach a prompt even if that filter breaks.
		if r.UserID != uid {
			continue
		}
		out = append(out, prompts.UserFact{
			Fact:         r.Fact,
			Category:     r.Category,
			Confidence:   r.Confidence,
			UserAuthored: r.UserAuthored,
		})
	}
	return out, nil
}

// Candidate is a proposed fact, from the extractor or from a user edit.
type Candidate struct {
	Fact       string
	Category   string
	Confidence float32
	// SourceMessageID is the turn that produced this. Empty for user edits.
	SourceMessageID string
	// UserAuthored marks a fact the user wrote or corrected themselves. These
	// bypass the confidence floor: the user is authoritative about themselves.
	UserAuthored bool
	// SupersedesID, when set, is the fact this one replaces.
	SupersedesID string
}

// ValidCategories is the closed vocabulary, mirroring the user_facts CHECK
// constraint. Exported so the extractor and the HTTP boundary validate against
// one list rather than two that drift.
var ValidCategories = map[string]bool{
	"commitment": true,
	"preference": true,
	"constraint": true,
	"context":    true,
}

// maxFactChars bounds a stored fact. A "fact" longer than this is a summary,
// and summaries belong in the search index, not the curated tier.
const maxFactChars = 500

// storeConfidenceFloor is the floor for *persisting* a model-extracted fact,
// as opposed to injecting one. Deliberately lower than MinConfidence: a
// moderately-confident fact is worth keeping so the user can confirm or correct
// it, but not worth acting on until it clears the injection threshold.
const storeConfidenceFloor = 0.3

// Validate reports why a candidate is unfit to store, or nil if it is fit.
func (c Candidate) Validate() error {
	fact := strings.TrimSpace(c.Fact)
	switch {
	case fact == "":
		return fmt.Errorf("fact is empty")
	case len(fact) > maxFactChars:
		return fmt.Errorf("fact exceeds %d characters", maxFactChars)
	case !ValidCategories[c.Category]:
		return fmt.Errorf("unsupported category %q", c.Category)
	case c.Confidence < 0 || c.Confidence > 1:
		return fmt.Errorf("confidence %v out of range", c.Confidence)
	case !c.UserAuthored && c.Confidence < storeConfidenceFloor:
		// This is the guard against the failure the roadmap names explicitly:
		// a low-confidence model guess becoming a durable "fact" about someone.
		return fmt.Errorf("confidence %.2f below store floor %.2f", c.Confidence, storeConfidenceFloor)
	}
	return nil
}

// Record persists a candidate, superseding the fact it replaces when one is
// named. Returns the stored fact, or ok=false when the fact was already known
// (the live-uniqueness index rejected it) — which is a no-op, not an error,
// because the extractor re-proposes known facts on every turn.
func (s *Store) Record(ctx context.Context, userID string, c Candidate) (db.UserFact, bool, error) {
	if s == nil {
		return db.UserFact{}, false, nil
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return db.UserFact{}, false, fmt.Errorf("facts: invalid user id: %w", err)
	}
	if err := c.Validate(); err != nil {
		return db.UserFact{}, false, fmt.Errorf("facts: %w", err)
	}

	var sourceID uuid.NullUUID
	if c.SourceMessageID != "" {
		parsed, err := uuid.Parse(c.SourceMessageID)
		if err != nil {
			return db.UserFact{}, false, fmt.Errorf("facts: invalid source message id: %w", err)
		}
		sourceID = uuid.NullUUID{UUID: parsed, Valid: true}
	}

	created, err := s.q.CreateUserFact(ctx, db.CreateUserFactParams{
		UserID:          uid,
		Fact:            strings.TrimSpace(c.Fact),
		Category:        c.Category,
		Confidence:      c.Confidence,
		SourceMessageID: sourceID,
		UserAuthored:    c.UserAuthored,
	})
	if err != nil {
		return db.UserFact{}, false, fmt.Errorf("facts: create: %w", err)
	}
	// ON CONFLICT DO NOTHING returns a zero row: the fact is already known.
	if created.ID == uuid.Nil {
		return db.UserFact{}, false, nil
	}

	// Link the replaced fact only after the replacement exists, so a failure
	// here leaves both facts live rather than leaving the user with neither.
	if c.SupersedesID != "" {
		oldID, err := uuid.Parse(c.SupersedesID)
		if err != nil {
			return created, true, fmt.Errorf("facts: invalid supersedes id: %w", err)
		}
		if _, err := s.q.SupersedeUserFact(ctx, oldID, uid, uuid.NullUUID{UUID: created.ID, Valid: true}); err != nil {
			return created, true, fmt.Errorf("facts: supersede %s: %w", oldID, err)
		}
	}

	return created, true, nil
}

// Forget deletes one fact. A hard delete, not a supersession: when someone asks
// the coach to forget something, keeping it as history defeats the request.
func (s *Store) Forget(ctx context.Context, userID, factID string) error {
	if s == nil {
		return nil
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("facts: invalid user id: %w", err)
	}
	fid, err := uuid.Parse(factID)
	if err != nil {
		return fmt.Errorf("facts: invalid fact id: %w", err)
	}
	if err := s.q.ForgetUserFact(ctx, fid, uid); err != nil {
		return fmt.Errorf("facts: forget: %w", err)
	}
	return nil
}

// ForgetAll deletes every fact for a user. Backs "disable long-term memory"
// and account deletion.
func (s *Store) ForgetAll(ctx context.Context, userID string) error {
	if s == nil {
		return nil
	}
	uid, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("facts: invalid user id: %w", err)
	}
	if err := s.q.ForgetAllUserFacts(ctx, uid); err != nil {
		return fmt.Errorf("facts: forget all: %w", err)
	}
	return nil
}
