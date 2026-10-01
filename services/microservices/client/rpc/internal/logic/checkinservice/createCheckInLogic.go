package checkinservicelogic

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/validator"
	goalslogic "github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/logic/goals"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// pgxTxRunner is satisfied by *postgres.PgxTxRunner; tests inject a no-op.
type pgxTxRunner interface {
	Run(ctx context.Context, userID string, fn func(pgx.Tx) error) error
}

type CreateCheckInLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
	// testTxRunner is only set in tests to inject a no-op transaction runner.
	testTxRunner pgxTxRunner
}

func NewCreateCheckInLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCheckInLogic {
	return &CreateCheckInLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *CreateCheckInLogic) getTxRunner() pgxTxRunner {
	if l.testTxRunner != nil {
		return l.testTxRunner
	}
	return l.svcCtx.TxRunner
}

// getTxRepo returns the repository to use inside the transaction. In
// production it binds to tx; when testTxRunner is set the tx is a nil stub and
// the mock repository from svcCtx.Repo is used instead.
func (l *CreateCheckInLogic) getTxRepo(tx pgx.Tx) *repository.Repository {
	if l.testTxRunner != nil {
		return l.svcCtx.Repo
	}
	return l.svcCtx.WithTx(tx)
}

func (l *CreateCheckInLogic) CreateCheckIn(in *client.CreateCheckInRequest) (*client.CreateCheckInResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "CreateCheckInLogic.CreateCheckIn")
	defer span.End()
	// Validate input
	if in.HabitId == "" || in.Status == "" {
		return nil, status.Error(codes.InvalidArgument, "habitId and status are required")
	}

	p, ok := principal.PrincipalFrom(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing principal")
	}
	userID, err := uuid.Parse(p.UserID)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.Internal, "invalid user id")
	}

	habitID, err := uuid.Parse(in.HabitId)
	if err != nil {
		l.Errorf("Invalid habit ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid habit ID")
	}

	// Validate enum values and length bounds before persistence / AI prompts.
	if in.Status != "completed" && in.Status != "missed" {
		return nil, status.Error(codes.InvalidArgument, "status must be 'completed' or 'missed'")
	}
	if in.Note != "" && !validator.MaxLength(in.Note, 1000) {
		return nil, status.Error(codes.InvalidArgument, "note exceeds maximum length of 1000 characters")
	}
	if in.Mood != "" && !validator.MaxLength(in.Mood, 50) {
		return nil, status.Error(codes.InvalidArgument, "mood exceeds maximum length of 50 characters")
	}
	if in.Energy != "" && !validator.MaxLength(in.Energy, 50) {
		return nil, status.Error(codes.InvalidArgument, "energy exceeds maximum length of 50 characters")
	}
	if in.Blocker != "" && !validator.MaxLength(in.Blocker, 200) {
		return nil, status.Error(codes.InvalidArgument, "blocker exceeds maximum length of 200 characters")
	}

	// Wrap all state-mutating operations — including the event outbox write —
	// in a transaction with RLS context so a check-in and its downstream event
	// commit or roll back together (no publish-after-commit loss window).
	var checkIn db.CheckIn
	var habit db.GetHabitRow
	var streak int32
	err = l.getTxRunner().Run(ctx, userID.String(), func(tx pgx.Tx) error {
		txRepo := l.getTxRepo(tx)

		timezone := "UTC"
		if prefs, pErr := txRepo.UserPreferences.GetUserPreferences(ctx, userID); pErr == nil {
			timezone = prefs.Timezone
		}

		// Verify the habit exists and belongs to the caller before creating a
		// check-in. Prevents IDOR (checking in on another user's habit).
		habit, err = txRepo.Habits.GetHabitByID(ctx, habitID, timezone)
		if err != nil {
			return status.Error(codes.NotFound, "habit not found")
		}
		if habit.UserID != userID {
			return status.Error(codes.PermissionDenied, "access denied")
		}

		// Idempotency (P3): fetch the current row for this habit/day before
		// upserting. A retried request that changes nothing produces no new
		// activity row and no new event — side effects only fire on a real
		// transition.
		var existing *db.CheckIn
		if prev, prevErr := txRepo.CheckIns.GetTodayCheckInByHabit(ctx, habitID, timezone); prevErr == nil {
			existing = &prev
		} else if !errors.Is(prevErr, pgx.ErrNoRows) {
			return fmt.Errorf("get existing check-in: %w", prevErr)
		}
		unchanged := existing != nil &&
			existing.Status == in.Status &&
			optStringEq(existing.Mood, in.Mood) &&
			optStringEq(existing.Energy, in.Energy) &&
			optStringEq(existing.Blocker, in.Blocker) &&
			optStringEq(existing.Note, in.Note)

		// Upsert check-in record. If a check-in already exists for today
		// (UNIQUE(habit_id, local_date)), update its status/mood/energy/
		// blocker/note instead of failing with 409. This lets users re-check-in
		// to change status (missed → completed) or add details after the fact.
		params := protoToUpsertCheckInParams(userID, habitID, in.Status, in.Mood, in.Energy, in.Blocker, in.Note)
		params.Timezone = timezone
		checkIn, err = txRepo.CheckIns.UpsertCheckIn(ctx, params)
		if err != nil {
			return fmt.Errorf("upsert check-in: %w", err)
		}

		// Streak is derived from check_ins history (consecutive completed days),
		// not a stored counter, so completion/miss no longer mutate it. Recompute
		// it here so the response and the published event carry the truthful
		// value (e.g. a 'completed' check-in may start/extend a streak; a
		// 'missed' check-in does not change today's streak).
		if s, sErr := txRepo.Habits.GetHabitStreak(ctx, habitID, userID, timezone); sErr == nil {
			streak = s
		}

		if !unchanged {
			// Log activity record. dedupe_key makes the insert race-safe: two
			// identical in-flight requests produce at most one activity row.
			activityType := "check_in_missed"
			activityTitle := fmt.Sprintf("Missed %s", habit.Name)
			if in.Status == "completed" {
				activityType = "check_in_completed"
				activityTitle = fmt.Sprintf("Completed %s", habit.Name)
			}
			description := fmt.Sprintf("Check-in %s for habit: %s", in.Status, habit.Name)
			if err := txRepo.Activities.CreateActivityDeduped(ctx, db.CreateActivityDedupedParams{
				Type:        activityType,
				Title:       activityTitle,
				Description: &description,
				Metadata:    json.RawMessage("{}"),
				UserID:      userID,
				DedupeKey:   strPtr(checkInActivityDedupeKey(checkIn.ID, in.Status, in.Mood, in.Energy, in.Blocker, in.Note)),
			}); err != nil {
				return fmt.Errorf("create activity: %w", err)
			}

			// Enqueue check_in_created into the outbox in this transaction.
			// The event ID is derived from (check_in_id, version): a redelivery
			// or relay replay is deduped downstream, and every real content
			// transition (version bump) yields a distinct ID. The payload
			// carries the user-authored fields so the ai-coach-consumer can
			// maintain its own read model without touching check_ins (P2).
			var localDate string
			if checkIn.LocalDate.Valid {
				localDate = checkIn.LocalDate.Time.Format("2006-01-02")
			}
			env, envErr := events.NewEnvelopeWithID(checkInEventID(checkIn.ID, checkIn.Version), events.TypeCheckInCreated, events.CheckInCreated{
				UserID:    userID.String(),
				CheckInID: checkIn.ID.String(),
				HabitID:   habit.ID.String(),
				HabitName: habit.Name,
				Status:    in.Status,
				Streak:    streak,
				LocalDate: localDate,
				Mood:      in.Mood,
				Energy:    in.Energy,
				Blocker:   in.Blocker,
				Note:      in.Note,
			})
			if envErr != nil {
				return fmt.Errorf("build check-in envelope: %w", envErr)
			}
			if err := txRepo.EventOutbox.Enqueue(ctx, env); err != nil {
				return fmt.Errorf("enqueue check-in event: %w", err)
			}
		}

		// Recompute progress for any habit-driven goals linked to this habit.
		// Only 'completed' check-ins move the needle (the habit formula counts
		// completed days), but we recompute on both statuses so a 'missed'
		// check-in that replaces a previous 'completed' for the same day is
		// handled correctly by the distinct-day count.
		linkedGoalIDs, gErr := txRepo.Goals.ListGoalIDsByHabit(ctx, habitID)
		if gErr != nil {
			return fmt.Errorf("list linked goals: %w", gErr)
		}
		for _, gid := range linkedGoalIDs {
			if _, rErr := goalslogic.RecomputeGoalProgressWithRepo(ctx, txRepo.Goals, txRepo.CheckIns, gid); rErr != nil {
				return fmt.Errorf("recompute goal %s progress: %w", gid, rErr)
			}
		}
		return nil
	})
	if err != nil {
		// Preserve business-logic gRPC status errors (e.g. AlreadyExists,
		// InvalidArgument, PermissionDenied) returned from inside the
		// transaction so the client receives the correct status code and
		// message. Only unexpected errors are logged and converted to Internal.
		if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
			return nil, st.Err()
		}
		l.Errorf("Failed check-in workflow: %v", err)
		return nil, status.Error(codes.Internal, "failed check-in workflow")
	}

	// Invalidate the cached personalization context so the next coaching
	// request reflects the new check-in immediately rather than at TTL.
	l.svcCtx.InvalidatePersonalizationContext(ctx, userID)

	return &client.CreateCheckInResponse{
		CheckIn:    checkInToProto(checkIn),
		Habit:      habitToProto(habit, streak),
		AiFeedback: "", // delivered asynchronously via notifications from the ai-coach-consumer
	}, nil
}

// optStringEq compares a nullable DB string column with a proto field where
// "" maps to NULL (protoToUpsertCheckInParams drops empty strings).
func optStringEq(dbValue *string, inValue string) bool {
	if dbValue == nil {
		return inValue == ""
	}
	return *dbValue == inValue
}

// checkInEventID derives the deterministic event ID for a check-in state
// transition: one unique ID per (check_in, version) so replays dedupe while
// every real change propagates.
func checkInEventID(checkInID uuid.UUID, version int32) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("check_in_created:%s:%d", checkInID, version))).String()
}

// checkInActivityDedupeKey is the idempotency key for the activity row a
// check-in writes. Identical requests (double-taps, retries) hash to the
// same key so only the first insert lands; a genuine content change produces
// a different key and a new activity entry.
func checkInActivityDedupeKey(checkInID uuid.UUID, status, mood, energy, blocker, note string) string {
	h := sha256.Sum256([]byte(status + "\x00" + mood + "\x00" + energy + "\x00" + blocker + "\x00" + note))
	return fmt.Sprintf("check-in:%s:%s", checkInID, hex.EncodeToString(h[:8]))
}

func strPtr(s string) *string {
	return &s
}
