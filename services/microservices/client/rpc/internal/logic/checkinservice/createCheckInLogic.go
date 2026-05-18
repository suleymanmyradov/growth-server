package checkinservicelogic

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

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
	// Validate input
	if in.HabitId == "" || in.Status == "" {
		return nil, status.Error(codes.InvalidArgument, "habitId and status are required")
	}

	habitID, err := uuid.Parse(in.HabitId)
	if err != nil {
		l.Errorf("Invalid habit ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid habit ID")
	}

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		l.Errorf("Invalid user ID: %v", err)
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Check for duplicate check-in
	alreadyCheckedIn, err := l.svcCtx.Repo.CheckIns.HasCheckedInToday(l.ctx, userID, habitID)
	if err != nil {
		l.Errorf("Failed to check existing check-in: %v", err)
		return nil, status.Error(codes.Internal, "failed to check existing check-in")
	}
	if alreadyCheckedIn {
		return nil, status.Error(codes.AlreadyExists, "already checked in on this habit today")
	}

	// Create check-in record
	params := protoToCheckInParams(in.UserId, in.HabitId, in.Status, in.Mood, in.Energy, in.Blocker, in.Note)
	checkIn, err := l.svcCtx.Repo.CheckIns.CreateCheckIn(l.ctx, params)
	if err != nil {
		l.Errorf("Failed to create check-in: %v", err)
		return nil, status.Error(codes.Internal, "failed to create check-in")
	}

	// Get habit to return in response
	habit, err := l.svcCtx.Repo.Habits.GetHabitByID(l.ctx, habitID)
	if err != nil {
		l.Errorf("Failed to get habit: %v", err)
		return nil, status.Error(codes.Internal, "failed to get habit")
	}

	// If completed, toggle habit to mark as completed and bump streak
	if in.Status == "completed" {
		updatedHabit, err := l.svcCtx.Repo.Habits.ToggleHabit(l.ctx, habitID)
		if err != nil {
			l.Errorf("Failed to toggle habit: %v", err)
		} else {
			habit = updatedHabit
		}
	} else if in.Status == "missed" {
		// Reset streak on missed check-in
		_, err := l.svcCtx.Repo.Habits.UpdateHabitStreak(l.ctx, habitID, 0)
		if err != nil {
			l.Errorf("Failed to reset habit streak: %v", err)
		}
	}

	// Log activity record
	activityType := "check_in_missed"
	activityTitle := fmt.Sprintf("Missed %s", habit.Name)
	if in.Status == "completed" {
		activityType = "check_in_completed"
		activityTitle = fmt.Sprintf("Completed %s", habit.Name)
	}

	_, err = l.svcCtx.Repo.Activities.CreateActivity(l.ctx, db.CreateActivityParams{
		ItemType:    activityType,
		Title:       activityTitle,
		Description: sql.NullString{String: fmt.Sprintf("Check-in %s for habit: %s", in.Status, habit.Name), Valid: true},
		Metadata:    pqtype.NullRawMessage{},
		UserID:      userID,
	})
	if err != nil {
		l.Errorf("Failed to create activity: %v", err)
	}

	// Fire-and-forget publish check-in event to Kafka.
	if l.svcCtx.EventsPub != nil {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			env, err := events.NewEnvelope(events.TypeCheckInCreated, events.CheckInCreated{
				UserID:    userID.String(),
				CheckInID: checkIn.ID.String(),
				HabitID:   habit.ID.String(),
				HabitName: habit.Name,
				Status:    in.Status,
				Streak:    habit.Streak.Int32,
			})
			if err != nil {
				logx.Errorf("envelope: %v", err)
				return
			}
			if err := l.svcCtx.EventsPub.Publish(ctx, env); err != nil {
				logx.Errorf("publish check-in event: %v", err)
			}
		}()
	}

	return &client.CreateCheckInResponse{
		CheckIn:    checkInToProto(checkIn),
		Habit:      habitToProto(habit),
		AiFeedback: "", // TODO: remove once frontend migrates to async delivery via notifications
	}, nil
}
