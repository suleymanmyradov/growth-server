package checkinservicelogic

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/pkg/validator"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/aicoachservice"
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
	err = l.svcCtx.TxRunner.Run(ctx, in.UserId, func(tx pgx.Tx) error {
		txRepo := l.svcCtx.WithTx(tx)

		// Check for duplicate check-in
		alreadyCheckedIn, err := txRepo.CheckIns.HasCheckedInToday(ctx, userID, habitID)
		if err != nil {
			return fmt.Errorf("check existing check-in: %w", err)
		}
		if alreadyCheckedIn {
			return status.Error(codes.AlreadyExists, "already checked in on this habit today")
		}

		// Create check-in record
		params := protoToCheckInParams(userID, habitID, in.Status, in.Mood, in.Energy, in.Blocker, in.Note)
		checkIn, err = txRepo.CheckIns.CreateCheckIn(ctx, params)
		if err != nil {
			return fmt.Errorf("create check-in: %w", err)
		}

		// Get habit to return in response
		habit, err = txRepo.Habits.GetHabitByID(ctx, habitID)
		if err != nil {
			return fmt.Errorf("get habit: %w", err)
		}

		// If completed, toggle habit to mark as completed and bump streak
		// The habit's completed flag derives from the check-in just written;
		// only the streak counter needs updating here.
		switch in.Status {
		case "completed":
			updatedHabit, err := txRepo.Habits.UpdateHabitStreak(ctx, habitID, habit.Streak+1)
			if err != nil {
				return fmt.Errorf("bump habit streak: %w", err)
			}
			habit = updatedHabit
		case "missed":
			// Reset streak on missed check-in
			_, err := txRepo.Habits.UpdateHabitStreak(ctx, habitID, 0)
			if err != nil {
				return fmt.Errorf("reset habit streak: %w", err)
			}
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
			Type:    (activityType),
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
		l.Errorf("Failed check-in workflow: %v", err)
return nil, status.Error(codes.Internal, "failed check-in workflow")
	}

	// Fire-and-forget publish check-in event to Kafka.
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
				Streak:    habit.Streak,
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

	// Fire-and-forget: generate AI feedback (non-blocking, best-effort)
	runBackground(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		feedback := l.generateAIFeedback(ctx, in, habit)
		if feedback != "" {
			l.Infof("AI feedback generated for check-in %s: %s", checkIn.ID, feedback)
		}
	})

	return &client.CreateCheckInResponse{
		CheckIn:    checkInToProto(checkIn),
		Habit:      habitToProto(habit),
		AiFeedback: "", // TODO: remove once frontend migrates to async delivery via notifications
	}, nil
}

func (l *CreateCheckInLogic) generateAIFeedback(ctx context.Context, in *client.CreateCheckInRequest, habit db.GetHabitRow) string {
	// Fetch user settings for accountability style
	settings, err := l.svcCtx.Repo.UserSettings.GetUserSettings(ctx, habit.UserID)
	accountabilityStyle := "balanced"
	if err == nil && settings.AccountabilityStyle != "" {
		accountabilityStyle = string(settings.AccountabilityStyle)
	}

	// Fetch the primary goal for context (best-effort)
	goals, err := l.svcCtx.Repo.Goals.ListGoals(ctx, habit.UserID, 1, 0)
	goalTitle := ""
	if err == nil && len(goals) > 0 {
		goalTitle = goals[0].Title
	}

	// Build recent pattern summary (last 7 days check-ins for this habit)
	weekAgo := time.Now().AddDate(0, 0, -7)
	recentCheckIns, err := l.svcCtx.Repo.CheckIns.GetCheckInsForWeek(ctx, habit.UserID, weekAgo, time.Now())
	recentPattern := "No recent check-ins"
	if err == nil && len(recentCheckIns) > 0 {
		completedCount := 0
		missedCount := 0
		for _, ci := range recentCheckIns {
			if ci.HabitID == habit.ID {
				if ci.Status == "completed" {
					completedCount++
				} else {
					missedCount++
				}
			}
		}
		if completedCount+missedCount > 0 {
			recentPattern = fmt.Sprintf("%d completed, %d missed in the last 7 days", completedCount, missedCount)
		}
	}

	resp, err := l.svcCtx.AICoachRpc.GenerateCheckInFeedback(ctx, &aicoachservice.CheckInFeedbackRequest{
		UserId:               in.UserId,
		HabitId:              in.HabitId,
		HabitName:            habit.Name,
		Status:               in.Status,
		Mood:                 in.Mood,
		Energy:               in.Energy,
		Blocker:              in.Blocker,
		Note:                 in.Note,
		Streak:               int32(habit.Streak),
		AccountabilityStyle:  accountabilityStyle,
		PreferredTone:        "",
		DifficultyPreference: "",
		CommonBlockers:       nil,
		RecentPattern:        recentPattern,
		GoalTitle:            goalTitle,
	})
	if err != nil {
		l.Errorf("AI feedback generation failed: %v", err)
		return ""
	}

	return resp.Feedback
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
