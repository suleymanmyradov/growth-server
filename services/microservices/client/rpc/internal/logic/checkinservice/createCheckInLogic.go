package checkinservicelogic

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/suleymanmyradov/growth-server/pkg/auth/principal"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/validator"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// backgroundSem caps the number of concurrent fire-and-forget goroutines
// spawned by CreateCheckIn to prevent goroutine exhaustion under load.
var backgroundSem = make(chan struct{}, 100)

func runBackground(f func()) {
	select {
	case backgroundSem <- struct{}{}:
		go func() {
			defer func() { <-backgroundSem }()
			f()
		}()
	default:
		logx.Error("background task dropped: semaphore full")
	}
}

type CreateCheckInLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewCreateCheckInLogic(ctx context.Context, svcCtx *svc.ServiceContext) *CreateCheckInLogic {
	return &CreateCheckInLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
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

	// Wrap all state-mutating operations in a transaction with RLS context.
	var checkIn db.CheckIn
	var habit db.GetHabitRow
	var streak int32
	err = l.svcCtx.TxRunner.Run(ctx, userID.String(), func(tx pgx.Tx) error {
		txRepo := l.svcCtx.WithTx(tx)

		// Verify the habit exists and belongs to the caller before creating a
		// check-in. Prevents IDOR (checking in on another user's habit).
		habit, err = txRepo.Habits.GetHabitByID(ctx, habitID)
		if err != nil {
			return status.Error(codes.NotFound, "habit not found")
		}
		if habit.UserID != userID {
			return status.Error(codes.PermissionDenied, "access denied")
		}

		// Check for duplicate check-in
		alreadyCheckedIn, err := txRepo.CheckIns.HasCheckedInToday(ctx, userID, habitID)
		if err != nil {
			return fmt.Errorf("check existing check-in: %w", err)
		}
		if alreadyCheckedIn {
			return status.Error(codes.AlreadyExists, "already checked in on this habit today")
		}

		// Create check-in record. The UNIQUE(habit_id, local_date) constraint is
		// the last line of defense against a concurrent duplicate; surface that
		// race as AlreadyExists (409) instead of a generic Internal (500).
		params := protoToCheckInParams(userID, habitID, in.Status, in.Mood, in.Energy, in.Blocker, in.Note)
		checkIn, err = txRepo.CheckIns.CreateCheckIn(ctx, params)
		if err != nil {
			var pgErr *pgconn.PgError
			if errors.As(err, &pgErr) && pgErr.Code == "23505" {
				return status.Error(codes.AlreadyExists, "already checked in on this habit today")
			}
			return fmt.Errorf("create check-in: %w", err)
		}

		// Streak is derived from check_ins history (consecutive completed days),
		// not a stored counter, so completion/miss no longer mutate it. Recompute
		// it here so the response and the published event carry the truthful
		// value (e.g. a 'completed' check-in may start/extend a streak; a
		// 'missed' check-in does not change today's streak).
		if s, sErr := txRepo.Habits.GetHabitStreak(ctx, habitID, userID); sErr == nil {
			streak = s
		}

		// Log activity record
		activityType := "check_in_missed"
		activityTitle := fmt.Sprintf("Missed %s", habit.Name)
		if in.Status == "completed" {
			activityType = "check_in_completed"
			activityTitle = fmt.Sprintf("Completed %s", habit.Name)
		}

		description := fmt.Sprintf("Check-in %s for habit: %s", in.Status, habit.Name)
		_, err = txRepo.Activities.CreateActivity(ctx, db.CreateActivityParams{
			Type:        (activityType),
			Title:       activityTitle,
			Description: &description,
			Metadata:    json.RawMessage("{}"),
			UserID:      userID,
		})
		if err != nil {
			return fmt.Errorf("create activity: %w", err)
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

	// Fire-and-forget publish check-in event to Kafka. The ai-coach-consumer
	// generates feedback asynchronously from this event, so there is no need
	// for a synchronous AI call here.
	if l.svcCtx.EventsPub != nil {
		runBackground(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			env, err := events.NewEnvelope(events.TypeCheckInCreated, events.CheckInCreated{
				UserID:    userID.String(),
				CheckInID: checkIn.ID.String(),
				HabitID:   habit.ID.String(),
				HabitName: habit.Name,
				Status:    in.Status,
				Streak:    streak,
			})
			if err != nil {
				logx.Errorf("envelope: %v", err)
				return
			}
			if err := l.svcCtx.EventsPub.Publish(ctx, env); err != nil {
				logx.Errorf("publish check-in event: %v", err)
			}
		})
	}

	return &client.CreateCheckInResponse{
		CheckIn:    checkInToProto(checkIn),
		Habit:      habitToProto(habit, streak),
		AiFeedback: "", // delivered asynchronously via notifications from the ai-coach-consumer
	}, nil
}
