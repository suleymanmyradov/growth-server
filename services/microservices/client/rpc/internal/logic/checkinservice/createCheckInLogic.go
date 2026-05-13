package checkinservicelogic

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/suleymanmyradov/growth-server/pkg/ai"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/prompts"
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

	// Generate AI coaching feedback
	aiFeedback := l.generateFeedback(userID, habit, in)

	return &client.CreateCheckInResponse{
		CheckIn:    checkInToProto(checkIn),
		Habit:      habitToProto(habit),
		AiFeedback: aiFeedback,
	}, nil
}

func (l *CreateCheckInLogic) generateFeedback(userID uuid.UUID, habit db.Habit, in *client.CreateCheckInRequest) string {
	if l.svcCtx.AI == nil {
		return ""
	}

	// Fetch user settings for accountability style
	accountabilityStyle := "balanced"
	settings, err := l.svcCtx.Repo.UserSettings.GetUserSettings(l.ctx, userID)
	if err == nil && settings.AccountabilityStyle != "" {
		accountabilityStyle = settings.AccountabilityStyle
	}

	// Fetch recent 7-day check-in pattern for this habit
	recentPattern := l.buildRecentPattern(userID, habit.ID)

	promptInput := prompts.CheckInFeedbackInput{
		HabitName:           habit.Name,
		Status:              in.Status,
		Mood:                in.Mood,
		Energy:              in.Energy,
		Blocker:             in.Blocker,
		Note:                in.Note,
		AccountabilityStyle: accountabilityStyle,
		Streak:              habit.Streak.Int32,
		RecentPattern:       recentPattern,
	}

	system := prompts.BuildSystemPrompt(accountabilityStyle)
	user := prompts.BuildUserPrompt(promptInput)

	resp, err := l.svcCtx.AI.Generate(l.ctx, ai.GenerateRequest{
		ModelProfile: ai.ModelCheap,
		System:       system,
		Messages: []ai.Message{
			{Role: ai.RoleUser, Content: user},
		},
		Metadata: ai.Metadata{
			UserID:  in.UserId,
			Feature: "check-in-feedback",
		},
	})
	if err != nil {
		l.Errorf("AI feedback generation failed: %v", err)
		return ""
	}

	return resp.Message.Content
}

func (l *CreateCheckInLogic) buildRecentPattern(userID, habitID uuid.UUID) string {
	now := time.Now().UTC()
	start := now.AddDate(0, 0, -7)
	checkIns, err := l.svcCtx.Repo.CheckIns.GetCheckInsForWeek(l.ctx, userID, start, now)
	if err != nil {
		return ""
	}

	var habitChecks int
	var completed int
	for _, c := range checkIns {
		if c.HabitID == habitID {
			habitChecks++
			if c.Status == "completed" {
				completed++
			}
		}
	}
	if habitChecks == 0 {
		return ""
	}
	return fmt.Sprintf("completed %d of last %d check-ins", completed, habitChecks)
}
