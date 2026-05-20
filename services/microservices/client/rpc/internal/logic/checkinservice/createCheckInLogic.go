package checkinservicelogic

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/sqlc-dev/pqtype"
	"github.com/suleymanmyradov/growth-server/pkg/ai"
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

	// Fire-and-forget: generate AI feedback (non-blocking, best-effort)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		feedback := l.generateAIFeedback(ctx, in, habit, checkIn)
		if feedback != "" {
			l.Infof("AI feedback generated for check-in %s: %s", checkIn.ID, feedback)
		}
	}()

	return &client.CreateCheckInResponse{
		CheckIn:    checkInToProto(checkIn),
		Habit:      habitToProto(habit),
		AiFeedback: "", // TODO: remove once frontend migrates to async delivery via notifications
	}, nil
}

func (l *CreateCheckInLogic) generateAIFeedback(ctx context.Context, in *client.CreateCheckInRequest, habit db.Habit, checkIn db.CheckIn) string {
	// Fetch user settings for accountability style
	settings, err := l.svcCtx.Repo.UserSettings.GetUserSettings(ctx, habit.UserID)
	accountabilityStyle := "balanced"
	if err == nil && settings.AccountabilityStyle != "" {
		accountabilityStyle = settings.AccountabilityStyle
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

	// Build the prompt
	systemPrompt := `You are an AI accountability coach. The user just checked in on their habit.

Respond with 2-3 sentences that are:
- Specific to this habit and situation
- Match the accountability tone specified
- Actionable (suggest a concrete next step)
- Never judgmental or shaming

If completed: acknowledge the win, reinforce the streak, suggest keeping momentum.
If missed: understand the blocker, suggest a small adjustment, protect tomorrow.`

	userMessage := fmt.Sprintf(`Context:
- Goal: %s
- Habit: %s
- Status: %s
- Mood: %s
- Energy: %s
- Blocker (if missed): %s
- Note: %s
- Accountability style: %s
- Streak: %d days
- Recent pattern: %s`,
		orDefault(goalTitle, "Not set"),
		habit.Name,
		in.Status,
		orDefault(in.Mood, "Not specified"),
		orDefault(in.Energy, "Not specified"),
		orDefault(in.Blocker, "None"),
		orDefault(in.Note, "None"),
		accountabilityStyle,
		habit.Streak.Int32,
		recentPattern,
	)

	resp, err := l.svcCtx.AIClient.Generate(ctx, ai.GenerateRequest{
		ModelProfile: ai.ModelCheap,
		System:       systemPrompt,
		Messages:     []ai.Message{{Role: ai.RoleUser, Content: userMessage}},
		Metadata:     ai.Metadata{UserID: habit.UserID.String(), Feature: "check_in_feedback"},
	})
	if err != nil {
		l.Errorf("AI feedback generation failed: %v", err)
		return ""
	}

	return resp.Message.Content
}

func orDefault(s, def string) string {
	if s == "" {
		return def
	}
	return s
}
