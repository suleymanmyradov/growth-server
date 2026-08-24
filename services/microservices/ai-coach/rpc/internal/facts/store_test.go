package facts

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/repository/db"
)

const (
	testUserID  = "8f14e45f-ea0c-4f9b-9a1e-2b3c4d5e6f70"
	otherUserID = "1c2d3e4f-5a6b-4c7d-8e9f-0a1b2c3d4e5f"
)

// fakeQueries records calls and returns canned rows.
type fakeQueries struct {
	listRows []db.UserFact
	listErr  error
	listArgs struct {
		userID     uuid.UUID
		confidence float32
		limit      int32
	}

	created     db.UserFact
	createErr   error
	createCalls []db.CreateUserFactParams

	supersedeErr   error
	supersedeCalls []uuid.UUID

	forgotten    []uuid.UUID
	forgotAllFor []uuid.UUID
}

func (f *fakeQueries) ListCurrentUserFacts(_ context.Context, userID uuid.UUID, confidence float32, limit int32) ([]db.UserFact, error) {
	f.listArgs.userID = userID
	f.listArgs.confidence = confidence
	f.listArgs.limit = limit
	return f.listRows, f.listErr
}

func (f *fakeQueries) CreateUserFact(_ context.Context, arg db.CreateUserFactParams) (db.UserFact, error) {
	f.createCalls = append(f.createCalls, arg)
	return f.created, f.createErr
}

func (f *fakeQueries) SupersedeUserFact(_ context.Context, id uuid.UUID, _ uuid.UUID, _ uuid.NullUUID) (db.UserFact, error) {
	f.supersedeCalls = append(f.supersedeCalls, id)
	return db.UserFact{}, f.supersedeErr
}

func (f *fakeQueries) ForgetUserFact(_ context.Context, id uuid.UUID, _ uuid.UUID) error {
	f.forgotten = append(f.forgotten, id)
	return nil
}

func (f *fakeQueries) ForgetAllUserFacts(_ context.Context, userID uuid.UUID) error {
	f.forgotAllFor = append(f.forgotAllFor, userID)
	return nil
}

func TestNewStoreDefaults(t *testing.T) {
	s := NewStore(&fakeQueries{}, Config{})
	if s.minConfidence != 0.5 {
		t.Errorf("default minConfidence = %v, want 0.5", s.minConfidence)
	}
	if s.maxFacts != 20 {
		t.Errorf("default maxFacts = %v, want 20", s.maxFacts)
	}
	if NewStore(nil, Config{}) != nil {
		t.Error("NewStore(nil) should return nil so callers can treat it as disabled")
	}
}

// A nil Store is the "curated memory disabled" case and must be safe to call.
func TestNilStoreIsSafe(t *testing.T) {
	var s *Store
	if got, err := s.ForPrompt(context.Background(), testUserID); err != nil || got != nil {
		t.Errorf("ForPrompt on nil store = %v, %v", got, err)
	}
	if _, ok, err := s.Record(context.Background(), testUserID, Candidate{}); err != nil || ok {
		t.Errorf("Record on nil store = %v, %v", ok, err)
	}
	if err := s.Forget(context.Background(), testUserID, testUserID); err != nil {
		t.Errorf("Forget on nil store = %v", err)
	}
	if err := s.ForgetAll(context.Background(), testUserID); err != nil {
		t.Errorf("ForgetAll on nil store = %v", err)
	}
}

func TestForPromptPassesConfidenceFloorAndLimit(t *testing.T) {
	q := &fakeQueries{}
	s := NewStore(q, Config{MinConfidence: 0.7, MaxFacts: 5})
	if _, err := s.ForPrompt(context.Background(), testUserID); err != nil {
		t.Fatalf("ForPrompt: %v", err)
	}
	if q.listArgs.confidence != 0.7 {
		t.Errorf("confidence floor = %v, want 0.7", q.listArgs.confidence)
	}
	if q.listArgs.limit != 5 {
		t.Errorf("limit = %v, want 5", q.listArgs.limit)
	}
	if q.listArgs.userID.String() != testUserID {
		t.Errorf("userID = %v", q.listArgs.userID)
	}
}

// Defense-in-depth: the query filters by user_id, but a row belonging to
// someone else must never reach a prompt even if that filter regresses.
func TestForPromptDropsOtherUsersFacts(t *testing.T) {
	own := uuid.MustParse(testUserID)
	other := uuid.MustParse(otherUserID)
	q := &fakeQueries{listRows: []db.UserFact{
		{UserID: other, Fact: "someone else's private fact", Category: "context"},
		{UserID: own, Fact: "trains Tuesdays and Thursdays", Category: "commitment"},
	}}
	s := NewStore(q, Config{})

	got, err := s.ForPrompt(context.Background(), testUserID)
	if err != nil {
		t.Fatalf("ForPrompt: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected 1 fact, got %d: %+v", len(got), got)
	}
	if got[0].Fact != "trains Tuesdays and Thursdays" {
		t.Errorf("wrong fact survived: %q", got[0].Fact)
	}
}

func TestForPromptRejectsNonUUIDUserID(t *testing.T) {
	q := &fakeQueries{}
	s := NewStore(q, Config{})
	if _, err := s.ForPrompt(context.Background(), "not-a-uuid"); err == nil {
		t.Error("expected error for non-UUID user id")
	}
}

// The failure this guards is the roadmap's own requirement: a low-confidence
// model guess must not become a durable fact about someone.
func TestCandidateValidate(t *testing.T) {
	tests := []struct {
		name    string
		c       Candidate
		wantErr bool
	}{
		{"valid", Candidate{Fact: "trains Tue/Thu", Category: "commitment", Confidence: 0.9}, false},
		{"empty fact", Candidate{Fact: "   ", Category: "commitment", Confidence: 0.9}, true},
		{"bad category", Candidate{Fact: "x", Category: "mood", Confidence: 0.9}, true},
		{"empty category", Candidate{Fact: "x", Confidence: 0.9}, true},
		{"confidence above range", Candidate{Fact: "x", Category: "context", Confidence: 1.5}, true},
		{"confidence below range", Candidate{Fact: "x", Category: "context", Confidence: -0.1}, true},
		{"low confidence guess rejected", Candidate{Fact: "x", Category: "context", Confidence: 0.1}, true},
		{"user-authored bypasses the floor", Candidate{Fact: "x", Category: "context", Confidence: 0, UserAuthored: true}, false},
		{"at the store floor", Candidate{Fact: "x", Category: "context", Confidence: 0.3}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.c.Validate()
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestRecordRejectsInvalidCandidateBeforeWriting(t *testing.T) {
	q := &fakeQueries{}
	s := NewStore(q, Config{})
	if _, _, err := s.Record(context.Background(), testUserID, Candidate{Fact: "x", Category: "mood", Confidence: 0.9}); err == nil {
		t.Error("expected validation error")
	}
	if len(q.createCalls) != 0 {
		t.Errorf("invalid candidate must not reach the database, got %d writes", len(q.createCalls))
	}
}

// The extractor re-proposes known facts every turn, so a uniqueness conflict is
// a no-op rather than an error.
func TestRecordAlreadyKnownIsNoOpNotError(t *testing.T) {
	q := &fakeQueries{created: db.UserFact{}} // zero row == ON CONFLICT DO NOTHING
	s := NewStore(q, Config{})
	got, ok, err := s.Record(context.Background(), testUserID, Candidate{
		Fact: "trains Tue/Thu", Category: "commitment", Confidence: 0.9,
	})
	if err != nil {
		t.Fatalf("already-known fact should not error: %v", err)
	}
	if ok {
		t.Error("ok should be false when the fact was already known")
	}
	if got.ID != uuid.Nil {
		t.Errorf("expected zero fact, got %+v", got)
	}
}

// Supersession must happen only after the replacement exists, so a failure
// leaves both facts live rather than leaving the user with neither.
func TestRecordSupersedesOnlyAfterInsertSucceeds(t *testing.T) {
	oldID := uuid.New()

	t.Run("insert fails: nothing superseded", func(t *testing.T) {
		q := &fakeQueries{createErr: errors.New("db down")}
		s := NewStore(q, Config{})
		if _, _, err := s.Record(context.Background(), testUserID, Candidate{
			Fact: "quit smoking again", Category: "commitment", Confidence: 0.9,
			SupersedesID: oldID.String(),
		}); err == nil {
			t.Fatal("expected error")
		}
		if len(q.supersedeCalls) != 0 {
			t.Errorf("must not supersede when the replacement was not stored")
		}
	})

	t.Run("insert succeeds: old fact superseded", func(t *testing.T) {
		q := &fakeQueries{created: db.UserFact{ID: uuid.New()}}
		s := NewStore(q, Config{})
		_, ok, err := s.Record(context.Background(), testUserID, Candidate{
			Fact: "started smoking again", Category: "context", Confidence: 0.9,
			SupersedesID: oldID.String(),
		})
		if err != nil || !ok {
			t.Fatalf("Record: ok=%v err=%v", ok, err)
		}
		if len(q.supersedeCalls) != 1 || q.supersedeCalls[0] != oldID {
			t.Errorf("expected supersede of %v, got %v", oldID, q.supersedeCalls)
		}
	})

	t.Run("already known: nothing superseded", func(t *testing.T) {
		q := &fakeQueries{created: db.UserFact{}}
		s := NewStore(q, Config{})
		if _, ok, err := s.Record(context.Background(), testUserID, Candidate{
			Fact: "x", Category: "context", Confidence: 0.9, SupersedesID: oldID.String(),
		}); err != nil || ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		if len(q.supersedeCalls) != 0 {
			t.Error("a no-op insert must not retire the previous fact")
		}
	})
}

func TestForgetAndForgetAll(t *testing.T) {
	factID := uuid.New()
	q := &fakeQueries{}
	s := NewStore(q, Config{})

	if err := s.Forget(context.Background(), testUserID, factID.String()); err != nil {
		t.Fatalf("Forget: %v", err)
	}
	if len(q.forgotten) != 1 || q.forgotten[0] != factID {
		t.Errorf("expected delete of %v, got %v", factID, q.forgotten)
	}

	if err := s.ForgetAll(context.Background(), testUserID); err != nil {
		t.Fatalf("ForgetAll: %v", err)
	}
	if len(q.forgotAllFor) != 1 || q.forgotAllFor[0].String() != testUserID {
		t.Errorf("expected forget-all for %v, got %v", testUserID, q.forgotAllFor)
	}

	if err := s.Forget(context.Background(), testUserID, "not-a-uuid"); err == nil {
		t.Error("expected error for non-UUID fact id")
	}
}
