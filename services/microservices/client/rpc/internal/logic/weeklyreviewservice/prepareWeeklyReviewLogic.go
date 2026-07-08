package weeklyreviewservicelogic

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type PrepareWeeklyReviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
	weeklyStatsLogic
}

func NewPrepareWeeklyReviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *PrepareWeeklyReviewLogic {
	return &PrepareWeeklyReviewLogic{
		ctx:              ctx,
		svcCtx:           svcCtx,
		Logger:           logx.WithContext(ctx),
		weeklyStatsLogic: weeklyStatsLogic{svcCtx: svcCtx, Logger: logx.WithContext(ctx)},
	}
}

// PrepareWeeklyReview computes the stats, patterns, and coaching preferences
// needed to build an AI weekly review request. If a cached review already
// exists (and forceRegenerate is false), it returns the existing review
// directly. If forceRegenerate is true and a cooldown is active, it returns a
// ResourceExhausted error.
func (l *PrepareWeeklyReviewLogic) PrepareWeeklyReview(in *client.PrepareWeeklyReviewRequest) (*client.PrepareWeeklyReviewResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "PrepareWeeklyReviewLogic.PrepareWeeklyReview")
	defer span.End()

	userID, err := uuid.Parse(in.UserId)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid user ID")
	}

	// Get user timezone
	settings, err := l.svcCtx.Repo.UserSettings.GetUserSettings(ctx, userID)
	if err != nil {
		l.Infof("failed to get user settings, using UTC: %v", err)
	}

	loc := time.UTC
	if settings.Timezone != "" {
		var err error
		loc, err = loadLocationCached(settings.Timezone)
		if err != nil {
			l.Infof("invalid timezone %s, using UTC: %v", settings.Timezone, err)
			loc = time.UTC
		}
	}

	weekStart, weekEnd, err := resolveWeekBounds(in.WeekStart, loc)
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, "invalid weekStart")
	}

	// Cache / cooldown check (same as the old Generate path).
	if !in.ForceRegenerate {
		existing, err := l.svcCtx.Repo.WeeklyReviews.GetWeeklyReview(ctx, userID, weekStart)
		if err == nil && existing.ID != uuid.Nil {
			return &client.PrepareWeeklyReviewResponse{
				ExistingReview: dbReviewToProto(existing),
			}, nil
		}
	} else {
		cooldown := l.svcCtx.Config.WeeklyReview.RegenerationCooldown
		if cooldown == 0 {
			cooldown = time.Hour
		}
		existing, err := l.svcCtx.Repo.WeeklyReviews.GetWeeklyReview(ctx, userID, weekStart)
		if err == nil && existing.ID != uuid.Nil {
			timeSinceGeneration := time.Since(existing.GeneratedAt.Time)
			if timeSinceGeneration < cooldown {
				remaining := cooldown - timeSinceGeneration
				return nil, status.Errorf(codes.ResourceExhausted, "please wait %s before regenerating", remaining.Round(time.Second))
			}
		}
	}

	// Compute stats
	stats, err := l.computeWeeklyStats(ctx, userID, weekStart, weekEnd)
	if err != nil {
		l.Errorf("compute weekly stats: %v", err)
		return nil, status.Error(codes.Internal, "failed to compute weekly stats")
	}

	// Fetch raw check-ins and habits for pattern detection
	weekCheckIns, err := fetchAllPages(checkInPageSize, func(limit, offset int32) ([]db.CheckIn, error) {
		return l.svcCtx.Repo.CheckIns.GetCheckInHistory(ctx, userID, weekStart, weekEnd, limit, offset)
	})
	if err != nil {
		l.Infof("failed to get all week check-ins for pattern detection (using %d fetched): %v", len(weekCheckIns), err)
	}
	if weekCheckIns == nil {
		weekCheckIns = []db.CheckIn{}
	}

	weekHabits, err := fetchAllPages(habitPageSize, func(limit, offset int32) ([]db.GetHabitRow, error) {
		return l.svcCtx.Repo.Habits.ListHabits(ctx, userID, limit, offset)
	})
	if err != nil {
		l.Infof("failed to get all habits for pattern detection (using %d fetched): %v", len(weekHabits), err)
	}
	if weekHabits == nil {
		weekHabits = []db.GetHabitRow{}
	}

	streakRows, err := l.svcCtx.Repo.Habits.GetHabitStreaks(ctx, userID)
	if err != nil {
		l.Infof("failed to get habit streaks: %v", err)
		streakRows = []db.GetHabitStreaksRow{}
	}
	streakByHabit := make(map[uuid.UUID]int32, len(streakRows))
	for _, s := range streakRows {
		streakByHabit[s.HabitID] = s.Streak
	}

	patternInsights := l.svcCtx.PatternDetection.AnalyzeFullFromData(weekCheckIns, weekHabits, streakByHabit, loc)

	// Coaching preferences
	preferredTone := "supportive"
	difficultyPreference := "adaptive"
	commonBlockers := []string{}

	coachingProfile, err := l.svcCtx.Repo.CoachingProfiles.GetCoachingProfile(ctx, userID)
	if err == nil && coachingProfile.UserID != uuid.Nil {
		if coachingProfile.PreferredTone != "" {
			preferredTone = string(coachingProfile.PreferredTone)
		}
		if coachingProfile.DifficultyPreference != "" {
			difficultyPreference = string(coachingProfile.DifficultyPreference)
		}
		if len(coachingProfile.CommonBlockers) > 0 {
			var blockers []string
			if err := json.Unmarshal(coachingProfile.CommonBlockers, &blockers); err == nil {
				commonBlockers = blockers
			}
		}
	}

	accountabilityStyle := "balanced"
	if settings.AccountabilityStyle != "" {
		accountabilityStyle = string(settings.AccountabilityStyle)
	}

	goals, err := fetchAllPages(goalPageSize, func(limit, offset int32) ([]db.GetGoalRow, error) {
		return l.svcCtx.Repo.Goals.ListGoals(ctx, userID, limit, offset)
	})
	if err != nil {
		l.Infof("failed to get all goals (using %d fetched): %v", len(goals), err)
	}

	goalTitles := make([]string, len(goals))
	for i, g := range goals {
		goalTitles[i] = g.Title
	}

	detectedPatterns := make([]string, 0, 6+len(patternInsights.RiskFactors))
	if patternInsights.CompletionPattern != "" {
		detectedPatterns = append(detectedPatterns, "Completion pattern: "+patternInsights.CompletionPattern)
	}
	if patternInsights.BestTimeOfDay != "" {
		detectedPatterns = append(detectedPatterns, "Best time of day: "+patternInsights.BestTimeOfDay)
	}
	if patternInsights.HardestTimeOfDay != "" {
		detectedPatterns = append(detectedPatterns, "Hardest time of day: "+patternInsights.HardestTimeOfDay)
	}
	if patternInsights.MoodEnergyCorrelation != "" && patternInsights.MoodEnergyCorrelation != "no_data" {
		detectedPatterns = append(detectedPatterns, "Mood/energy correlation: "+patternInsights.MoodEnergyCorrelation)
	}
	if patternInsights.StreakPattern != "" && patternInsights.StreakPattern != "no_data" {
		detectedPatterns = append(detectedPatterns, "Streak pattern: "+patternInsights.StreakPattern)
	}
	for _, risk := range patternInsights.RiskFactors {
		detectedPatterns = append(detectedPatterns, "Risk factor: "+risk)
	}

	// Build habit breakdowns (reusing the existing WeeklyReviewHabitBreakdown proto)
	habitBreakdowns := make([]*client.WeeklyReviewHabitBreakdown, len(stats.habitBreakdownsForDB))
	for i, h := range stats.habitBreakdownsForDB {
		var lastCheckInAt int64
		if t, err := time.Parse(time.RFC3339, h.LastCheckInAt); err == nil {
			lastCheckInAt = t.Unix()
		}
		habitBreakdowns[i] = &client.WeeklyReviewHabitBreakdown{
			HabitId:        h.HabitID,
			HabitName:      h.HabitName,
			Category:       h.Category,
			TotalCheckIns:  int32(h.TotalCheckIns),
			CompletedCount: int32(h.CompletedCount),
			MissedCount:    int32(h.MissedCount),
			CompletionRate: h.CompletionRate,
			LastCheckInAt:  lastCheckInAt,
		}
	}

	blockerStats := make([]*client.PreparedBlockerStat, len(stats.blockerStats))
	for i, b := range stats.blockerStats {
		blockerStats[i] = &client.PreparedBlockerStat{
			Blocker: b.Blocker,
			Count:   int32(b.Count),
		}
	}

	moodStats := make([]*client.PreparedMoodStat, len(stats.moodStats))
	for i, m := range stats.moodStats {
		moodStats[i] = &client.PreparedMoodStat{
			Mood:  m.Mood,
			Count: int32(m.Count),
		}
	}

	energyStats := make([]*client.PreparedEnergyStat, len(stats.energyStats))
	for i, e := range stats.energyStats {
		energyStats[i] = &client.PreparedEnergyStat{
			Energy: e.Energy,
			Count:  int32(e.Count),
		}
	}

	// moodSummary / energySummary for DB persistence
	moodSummary := make(map[string]int32, len(stats.moodMap))
	for k, v := range stats.moodMap {
		moodSummary[k] = int32(v)
	}
	energySummary := make(map[string]int32, len(stats.energyMap))
	for k, v := range stats.energyMap {
		energySummary[k] = int32(v)
	}

	return &client.PrepareWeeklyReviewResponse{
		Data: &client.PreparedWeeklyReviewData{
			UserId:               in.UserId,
			WeekStart:            weekStart.Format("2006-01-02"),
			AccountabilityStyle:  accountabilityStyle,
			PreferredTone:        preferredTone,
			DifficultyPreference: difficultyPreference,
			CommonBlockers:       commonBlockers,
			Goals:                goalTitles,
			TotalHabits:          int32(stats.totalHabits),
			CompletionRate:       stats.completionRate,
			CompletedCheckIns:    int32(stats.completedCheckIns),
			MissedCheckIns:       int32(stats.missedCheckIns),
			BestDay:              stats.bestDay,
			HardestDay:           stats.hardestDay,
			TopBlocker:           stats.topBlocker,
			HabitBreakdowns:      habitBreakdowns,
			BlockerStats:         blockerStats,
			MoodStats:            moodStats,
			EnergyStats:          energyStats,
			DetectedPatterns:     detectedPatterns,
			MoodSummary:          moodSummary,
			EnergySummary:        energySummary,
		},
	}, nil
}

// computeWeeklyStats is provided by the embedded weeklyStatsLogic.

