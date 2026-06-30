package personalizationservicelogic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
	gproto "google.golang.org/protobuf/proto"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type GetPersonalizationContextLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGetPersonalizationContextLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GetPersonalizationContextLogic {
	return &GetPersonalizationContextLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GetPersonalizationContextLogic) GetPersonalizationContext(in *client.GetPersonalizationContextRequest) (*client.GetPersonalizationContextResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GetPersonalizationContextLogic.GetPersonalizationContext")
	defer span.End()

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Check personalized AI entitlement
	sub, subErr := l.svcCtx.Repo.Billing.GetOrCreateUserSubscription(ctx, userID)
	if subErr == nil {
		entitlements, computeErr := l.svcCtx.Repo.Billing.ComputeEntitlements(ctx, sub, userID)
		if computeErr == nil && !entitlements.CanUsePersonalizedAi {
			// Return reduced/basic context for Free users
			profile, err := l.svcCtx.Repo.CoachingProfiles.GetCoachingProfile(ctx, userID)
			if err != nil {
				if errors.Is(err, pgx.ErrNoRows) {
					profile, _ = l.svcCtx.Repo.CoachingProfiles.UpsertCoachingProfile(ctx, db.UpsertCoachingProfileParams{
						UserID:              userID,
						AccountabilityStyle: "balanced",
						CoachTone:           "supportive",
						Difficulty:          "adaptive",
						CommonBlockers:      []byte("[]"),
						CoachingNotes:       []byte("{}"),
					})
				}
			}
			// Even on Free, include the user's name/bio so the coach can
			// address them personally. Non-fatal if unavailable.
			var freeUserProfile db.GetUserProfileByIDRow
			if up, upErr := l.svcCtx.Repo.Users.GetUserProfileByID(ctx, userID); upErr == nil {
				freeUserProfile = up
			}
			return &client.GetPersonalizationContextResponse{
				Context: &client.PersonalizationContext{
					Profile:            dbCoachingProfileToProto(profile),
					User:               dbUserProfileToProto(freeUserProfile),
					ActiveGoals:        []*client.Goal{},
					ActiveHabits:       []*client.Habit{},
					RecentCheckIns:     []*client.CheckIn{},
					PendingSuggestions: []*client.PlanAdjustmentSuggestion{},
					PatternInsights: map[string]string{
						"personalized_ai": "unavailable",
						"reason":          "Upgrade to Pro for personalized coaching context",
					},
				},
			}, nil
		}
	}

	// Pro path: serve the assembled personalization context from the Redis
	// read-through cache when fresh. This collapses ~11 DB queries (profile,
	// user, goals, habits, streaks, check-ins, weekly review, suggestions,
	// settings, goal-habit links) into a single Redis GET on the hot path;
	// singleflight dedupes concurrent misses so a cache stampede can't fan
	// out into 11*N queries. ForceRefresh bypasses the cache and repopulates
	// it so explicit refreshes always rebuild from DB. When Redis is not
	// configured, GetOrFetch falls back to calling the fetcher directly.
	cacheKey := svc.PersonalizationContextKey(userID)

	if in.ForceRefresh {
		pc, bErr := l.buildPersonalizationContext(ctx, userID, true)
		if bErr != nil {
			return nil, bErr
		}
		if data, mErr := gproto.Marshal(pc); mErr == nil {
			_ = l.svcCtx.Cache.Set(ctx, cacheKey, data, svc.PersonalizationContextTTL)
		}
		return &client.GetPersonalizationContextResponse{Context: pc}, nil
	}

	data, err := l.svcCtx.Cache.GetOrFetch(ctx, cacheKey, svc.PersonalizationContextTTL, func() ([]byte, error) {
		pc, bErr := l.buildPersonalizationContext(ctx, userID, false)
		if bErr != nil {
			return nil, bErr
		}
		return gproto.Marshal(pc)
	})
	if err != nil {
		// Preserve gRPC status errors from the builder (e.g. Internal);
		// only unexpected cache errors are wrapped.
		if st, ok := status.FromError(err); ok && st.Code() != codes.Unknown {
			return nil, st.Err()
		}
		return nil, status.Error(codes.Internal, "failed to build personalization context")
	}

	var pc client.PersonalizationContext
	if err := gproto.Unmarshal(data, &pc); err != nil {
		return nil, status.Error(codes.Internal, "failed to decode personalization context")
	}
	return &client.GetPersonalizationContextResponse{Context: &pc}, nil
}

// buildPersonalizationContext assembles the full personalized coaching context
// from the database. This is the cache-miss / refresh path: it runs the ~11
// queries and pattern detection, then returns the assembled proto. Callers are
// responsible for caching the result. forceRefresh records a context-refresh
// timestamp on the coaching profile.
func (l *GetPersonalizationContextLogic) buildPersonalizationContext(ctx context.Context, userID uuid.UUID, forceRefresh bool) (*client.PersonalizationContext, error) {
	// Get or create coaching profile
	profile, err := l.svcCtx.Repo.CoachingProfiles.GetCoachingProfile(ctx, userID)
	if err != nil {
		// Create default profile if it doesn't exist
		if errors.Is(err, sql.ErrNoRows) {
			profile, err = l.svcCtx.Repo.CoachingProfiles.UpsertCoachingProfile(ctx, db.UpsertCoachingProfileParams{
				UserID:              userID,
				AccountabilityStyle: "balanced",
				CoachTone:           "supportive",
				Difficulty:          "adaptive",
				CommonBlockers:      []byte("[]"),
				CoachingNotes:       []byte("{}"),
			})
			if err != nil {
				l.Errorf("failed to create default coaching profile: %v", err)
				return nil, status.Error(codes.Internal, "failed to create coaching profile")
			}
		} else {
			l.Errorf("failed to get coaching profile: %v", err)
			return nil, status.Error(codes.Internal, "failed to get coaching profile")
		}
	}

	// Get the user's public profile (name, bio, location, interests, etc.)
	// so coaching responses can address the user by name and reference
	// background context they've shared. Non-fatal: continue without it.
	userProfile, userErr := l.svcCtx.Repo.Users.GetUserProfileByID(ctx, userID)
	if userErr != nil {
		l.Infof("failed to get user profile: %v", userErr)
	}

	// Get active goals
	goals, err := l.svcCtx.Repo.Goals.ListGoals(ctx, userID, 50, 0)
	if err != nil {
		l.Infof("failed to get goals: %v", err)
		goals = []db.GetGoalRow{}
	}

	// Get active habits
	habits, err := l.svcCtx.Repo.Habits.ListHabits(ctx, userID, 50, 0)
	if err != nil {
		l.Infof("failed to get habits: %v", err)
		habits = []db.GetHabitRow{}
	}

	// Streaks are derived from check_ins history (not stored on the habit).
	streakRows, err := l.svcCtx.Repo.Habits.GetHabitStreaks(ctx, userID)
	if err != nil {
		l.Infof("failed to get habit streaks: %v", err)
		streakRows = []db.GetHabitStreaksRow{}
	}
	streakByHabit := make(map[uuid.UUID]int32, len(streakRows))
	for _, s := range streakRows {
		streakByHabit[s.HabitID] = s.Streak
	}

	// Get recent check-ins (last 30 days)
	thirtyDaysAgo := time.Now().AddDate(0, 0, -30)
	recentCheckIns, err := l.svcCtx.Repo.CheckIns.GetCheckInHistory(ctx, userID, thirtyDaysAgo, time.Now(), 100, 0)
	if err != nil {
		l.Infof("failed to get recent check-ins: %v", err)
		recentCheckIns = []db.CheckIn{}
	}

	// Get latest weekly review
	latestWeeklyReview, err := l.svcCtx.Repo.WeeklyReviews.ListWeeklyReviews(ctx, userID, 1, 0)
	if err != nil || len(latestWeeklyReview) == 0 {
		l.Infof("failed to get latest weekly review: %v", err)
		latestWeeklyReview = []db.GetWeeklyReviewRow{}
	}

	// Get pending plan adjustment suggestions
	pendingSuggestions, err := l.svcCtx.Repo.PlanAdjustmentSuggestions.ListPendingPlanAdjustmentSuggestions(ctx, userID, 20, 0)
	if err != nil {
		l.Infof("failed to get pending suggestions: %v", err)
		pendingSuggestions = []db.PlanAdjustment{}
	}

	// Get user timezone for pattern detection
	userLoc := time.UTC
	settings, err := l.svcCtx.Repo.UserSettings.GetUserSettings(ctx, userID)
	if err == nil && settings.Timezone != "" {
		loc, err := time.LoadLocation(settings.Timezone)
		if err != nil {
			l.Infof("invalid timezone %s, using UTC: %v", settings.Timezone, err)
		} else {
			userLoc = loc
		}
	}

	// Build pattern insights. The whole context is cached upstream, so pattern
	// detection only runs on a cache miss/refresh — no separate per-pattern
	// cache is needed here.
	patternInsights := l.svcCtx.PatternDetection.AnalyzeLite(recentCheckIns, habits, streakByHabit, userLoc)

	// Add habit count for backward compatibility
	if len(habits) > 0 {
		patternInsights["habit_count"] = strconv.Itoa(len(habits))
	}

	// Update context refresh timestamp if forced
	if forceRefresh {
		_, err = l.svcCtx.Repo.CoachingProfiles.UpdateCoachingProfileContextRefresh(ctx, userID)
		if err != nil {
			l.Infof("failed to update context refresh timestamp: %v", err)
		}
	}

	// Batch-fetch all goal-habit links for this user's goals and group by
	// goal ID so we can populate RelatedHabitIds without N+1 queries.
	linkRows, err := l.svcCtx.Repo.Goals.ListGoalHabitIDs(ctx, userID)
	if err != nil {
		l.Infof("failed to list goal-habit links: %v", err)
		linkRows = []db.ListGoalHabitIDsRow{}
	}
	habitsByGoal := make(map[uuid.UUID][]string, len(goals))
	for _, r := range linkRows {
		habitsByGoal[r.GoalID] = append(habitsByGoal[r.GoalID], r.HabitID.String())
	}

	// Build response
	protoProfile := dbCoachingProfileToProto(profile)
	protoGoals := make([]*client.Goal, len(goals))
	for i, goal := range goals {
		protoGoals[i] = dbGoalToProto(goal, habitsByGoal[goal.ID])
	}

	protoHabits := make([]*client.Habit, len(habits))
	for i, habit := range habits {
		protoHabits[i] = dbHabitToProto(habit, streakByHabit[habit.ID])
	}

	protoCheckIns := make([]*client.CheckIn, len(recentCheckIns))
	for i, checkIn := range recentCheckIns {
		protoCheckIns[i] = dbCheckInToProto(checkIn)
	}

	protoSuggestions := make([]*client.PlanAdjustmentSuggestion, len(pendingSuggestions))
	for i, suggestion := range pendingSuggestions {
		protoSuggestions[i] = dbPlanAdjustmentSuggestionToProto(suggestion)
	}

	var protoWeeklyReview *client.WeeklyReview
	if len(latestWeeklyReview) > 0 && latestWeeklyReview[0].ID != uuid.Nil {
		protoWeeklyReview = dbWeeklyReviewToProto(latestWeeklyReview[0])
	}

	return &client.PersonalizationContext{
		Profile:            protoProfile,
		User:               dbUserProfileToProto(userProfile),
		ActiveGoals:        protoGoals,
		ActiveHabits:       protoHabits,
		RecentCheckIns:     protoCheckIns,
		LatestWeeklyReview: protoWeeklyReview,
		PendingSuggestions: protoSuggestions,
		PatternInsights:    patternInsights,
	}, nil
}

func dbUserProfileToProto(u db.GetUserProfileByIDRow) *client.UserProfile {
	if u.ID == uuid.Nil {
		return nil
	}
	bio := ""
	if u.Bio != nil {
		bio = *u.Bio
	}
	location := ""
	if u.Location != nil {
		location = *u.Location
	}
	website := ""
	if u.Website != nil {
		website = *u.Website
	}
	avatarURL := ""
	if u.AvatarUrl != nil {
		avatarURL = *u.AvatarUrl
	}
	return &client.UserProfile{
		Id:        u.ID.String(),
		Username:  u.Username,
		FullName:  u.FullName,
		Bio:       bio,
		Location:  location,
		Website:   website,
		Interests: u.Interests,
		AvatarUrl: avatarURL,
	}
}

func dbGoalToProto(goal db.GetGoalRow, relatedHabitIds []string) *client.Goal {
	description := ""
	if goal.Description != nil {
		description = *goal.Description
	}
	dueDate := int64(0)
	if goal.DueDate.Valid {
		dueDate = goal.DueDate.Time.Unix()
	}
	if relatedHabitIds == nil {
		relatedHabitIds = []string{}
	}
	return &client.Goal{
		Id:              goal.ID.String(),
		Title:           goal.Title,
		Description:     description,
		Category:        goal.Category,
		DueDate:         dueDate,
		Progress:        goal.Progress,
		Completed:       goal.Completed,
		UserId:          goal.UserID.String(),
		CreatedAt:       goal.CreatedAt.Time.Unix(),
		UpdatedAt:       goal.UpdatedAt.Time.Unix(),
		RelatedHabitIds: relatedHabitIds,
	}
}

func dbHabitToProto(habit db.GetHabitRow, streak int32) *client.Habit {
	description := ""
	if habit.Description != nil {
		description = *habit.Description
	}
	return &client.Habit{
		Id:          habit.ID.String(),
		Name:        habit.Name,
		Description: description,
		Streak:      streak,
		Completed:   habit.Completed,
		Category:    habit.Category,
		UserId:      habit.UserID.String(),
		CreatedAt:   habit.CreatedAt.Time.Unix(),
		UpdatedAt:   habit.UpdatedAt.Time.Unix(),
	}
}

func dbCheckInToProto(checkIn db.CheckIn) *client.CheckIn {
	mood := ""
	if checkIn.Mood != nil {
		mood = string(*checkIn.Mood)
	}
	energy := ""
	if checkIn.Energy != nil {
		energy = string(*checkIn.Energy)
	}
	blocker := ""
	if checkIn.Blocker != nil {
		blocker = string(*checkIn.Blocker)
	}
	note := ""
	if checkIn.Note != nil {
		note = *checkIn.Note
	}
	return &client.CheckIn{
		Id:        checkIn.ID.String(),
		UserId:    checkIn.UserID.String(),
		HabitId:   checkIn.HabitID.String(),
		Status:    string(checkIn.Status),
		Mood:      mood,
		Energy:    energy,
		Blocker:   blocker,
		Note:      note,
		CreatedAt: checkIn.CreatedAt.Time.Unix(),
	}
}

func dbPlanAdjustmentSuggestionToProto(suggestion db.PlanAdjustment) *client.PlanAdjustmentSuggestion {
	var goalID, habitID string
	if suggestion.GoalID.Valid {
		goalID = suggestion.GoalID.UUID.String()
	}
	if suggestion.HabitID.Valid {
		habitID = suggestion.HabitID.UUID.String()
	}

	metadataJson := "{}"
	if suggestion.Metadata != nil {
		metadataJson = string(suggestion.Metadata)
	}

	return &client.PlanAdjustmentSuggestion{
		Id:             suggestion.ID.String(),
		UserId:         suggestion.UserID.String(),
		GoalId:         goalID,
		HabitId:        habitID,
		Source:         string(suggestion.Source),
		AdjustmentType: string(suggestion.AdjustmentType),
		Reason:         suggestion.Reason,
		Suggestion:     suggestion.Suggestion,
		Status:         string(suggestion.Status),
		MetadataJson:   metadataJson,
		CreatedAt:      suggestion.CreatedAt.Time.Unix(),
		UpdatedAt:      suggestion.UpdatedAt.Time.Unix(),
	}
}

func dbWeeklyReviewToProto(review db.GetWeeklyReviewRow) *client.WeeklyReview {
	// Parse completion rate from pgtype.Numeric to double
	completionRate := 0.0
	if review.CompletionRate.Valid {
		if _, err := fmt.Sscanf(fmt.Sprintf("%v", review.CompletionRate), "%f", &completionRate); err != nil {
			completionRate = 0.0
		}
	}

	// Parse habit breakdown from JSON
	var habitBreakdown []*client.WeeklyReviewHabitBreakdown
	if len(review.HabitBreakdown) > 0 {
		if err := json.Unmarshal(review.HabitBreakdown, &habitBreakdown); err != nil {
			logx.Errorf("failed to unmarshal habit breakdown: %v", err)
		}
	}

	// Parse suggested adjustments from JSON
	var suggestedAdjustments []*client.WeeklyReviewAdjustment
	if len(review.SuggestedAdjustments) > 0 {
		if err := json.Unmarshal(review.SuggestedAdjustments, &suggestedAdjustments); err != nil {
			logx.Errorf("failed to unmarshal suggested adjustments: %v", err)
		}
	}

	// Parse next week plan from JSON
	var nextWeekPlan *client.WeeklyReviewNextWeekPlan
	if len(review.NextWeekPlan) > 0 {
		if err := json.Unmarshal(review.NextWeekPlan, &nextWeekPlan); err != nil {
			logx.Errorf("failed to unmarshal next week plan: %v", err)
		}
	}

	// Parse mood summary from JSON
	var moodSummary map[string]int32
	if len(review.MoodSummary) > 0 {
		if err := json.Unmarshal(review.MoodSummary, &moodSummary); err != nil {
			logx.Errorf("failed to unmarshal mood summary: %v", err)
		}
	}

	// Parse energy summary from JSON
	var energySummary map[string]int32
	if len(review.EnergySummary) > 0 {
		if err := json.Unmarshal(review.EnergySummary, &energySummary); err != nil {
			logx.Errorf("failed to unmarshal energy summary: %v", err)
		}
	}

	bestDay := ""
	if review.BestDay != nil {
		bestDay = *review.BestDay
	}
	hardestDay := ""
	if review.HardestDay != nil {
		hardestDay = *review.HardestDay
	}
	topBlocker := ""
	if review.TopBlocker != nil {
		topBlocker = *review.TopBlocker
	}
	aiSummary := ""
	if review.AiSummary != nil {
		aiSummary = *review.AiSummary
	}

	return &client.WeeklyReview{
		Id:                   review.ID.String(),
		UserId:               review.UserID.String(),
		WeekStart:            review.WeekStart.Time.Format("2006-01-02"),
		WeekEnd:              review.WeekEnd.Time.Format("2006-01-02"),
		TotalHabits:          review.TotalHabits,
		CompletedCheckIns:    review.CompletedCheckIns,
		MissedCheckIns:       review.MissedCheckIns,
		CompletionRate:       completionRate,
		BestDay:              bestDay,
		HardestDay:           hardestDay,
		TopBlocker:           topBlocker,
		MoodSummary:          moodSummary,
		EnergySummary:        energySummary,
		HabitBreakdown:       habitBreakdown,
		AiSummary:            aiSummary,
		SuggestedAdjustments: suggestedAdjustments,
		NextWeekPlan:         nextWeekPlan,
		GeneratedAt:          review.GeneratedAt.Time.Unix(),
	}
}
