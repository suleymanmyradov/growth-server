package weeklyreviewservicelogic

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/pkg/prompts"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/aicoachservice"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	tzCacheMu sync.RWMutex
	tzCache   = make(map[string]*time.Location)
)

// loadLocationCached returns a *time.Location, caching successful lookups.
func loadLocationCached(name string) (*time.Location, error) {
	if name == "" || name == "UTC" {
		return time.UTC, nil
	}
	tzCacheMu.RLock()
	loc, ok := tzCache[name]
	tzCacheMu.RUnlock()
	if ok {
		return loc, nil
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return nil, err
	}
	tzCacheMu.Lock()
	tzCache[name] = loc
	tzCacheMu.Unlock()
	return loc, nil
}

type GenerateWeeklyReviewLogic struct {
	ctx    context.Context
	svcCtx *svc.ServiceContext
	logx.Logger
}

func NewGenerateWeeklyReviewLogic(ctx context.Context, svcCtx *svc.ServiceContext) *GenerateWeeklyReviewLogic {
	return &GenerateWeeklyReviewLogic{
		ctx:    ctx,
		svcCtx: svcCtx,
		Logger: logx.WithContext(ctx),
	}
}

func (l *GenerateWeeklyReviewLogic) GenerateWeeklyReview(in *client.GenerateWeeklyReviewRequest) (*client.GenerateWeeklyReviewResponse, error) {
	ctx, span := trace.TracerFromContext(l.ctx).Start(l.ctx, "GenerateWeeklyReviewLogic.GenerateWeeklyReview")
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

	if !in.ForceRegenerate {
		existing, err := l.svcCtx.Repo.WeeklyReviews.GetWeeklyReview(ctx, userID, weekStart)
		if err == nil && existing.ID != uuid.Nil {
			return &client.GenerateWeeklyReviewResponse{Review: dbReviewToProto(existing)}, nil
		}
	} else {
		// Add cooldown for forced regeneration (1 hour minimum)
		existing, err := l.svcCtx.Repo.WeeklyReviews.GetWeeklyReview(ctx, userID, weekStart)
		if err == nil && existing.ID != uuid.Nil {
			timeSinceGeneration := time.Since(existing.GeneratedAt.Time)
			if timeSinceGeneration < time.Hour {
				return nil, status.Error(codes.ResourceExhausted, "please wait at least 1 hour before regenerating")
			}
		}
	}

	stats, err := l.computeWeeklyStats(userID, weekStart, weekEnd)
	if err != nil {
		l.Errorf("compute weekly stats: %v", err)
		return nil, status.Error(codes.Internal, "failed to compute weekly stats")
	}

	// Get raw check-ins and habits for pattern detection
	weekCheckIns, err := l.svcCtx.Repo.CheckIns.GetCheckInHistory(ctx, userID, weekStart, weekEnd, 1000, 0)
	if err != nil {
		l.Infof("failed to get week check-ins for pattern detection: %v", err)
		weekCheckIns = []db.CheckIn{}
	}

	weekHabits, err := l.svcCtx.Repo.Habits.ListHabits(ctx, userID, 50, 0)
	if err != nil {
		l.Infof("failed to get habits for pattern detection: %v", err)
		weekHabits = []db.GetHabitRow{}
	}

	// Analyze patterns using the pattern detection service
	patternInsights := l.svcCtx.PatternDetection.AnalyzeFullFromData(weekCheckIns, weekHabits, loc)

	// Get personalization context for enhanced AI coaching
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

	goals, err := l.svcCtx.Repo.Goals.ListGoals(ctx, userID, 10, 0)
	if err != nil {
		l.Infof("failed to get goals: %v", err)
	}
	goalTitles := make([]string, len(goals))
	for i, g := range goals {
		goalTitles[i] = g.Title
	}

	// Build detected patterns from pattern insights
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

	// Log detected patterns for verification (debug level to avoid log noise)
	l.Debugf("Generated %d detected patterns for weekly review: %v", len(detectedPatterns), detectedPatterns)

	habitBreakdowns := make([]*aicoachservice.HabitBreakdown, len(stats.habitBreakdowns))
	for i, h := range stats.habitBreakdowns {
		habitBreakdowns[i] = &aicoachservice.HabitBreakdown{
			HabitId:        h.HabitID,
			HabitName:      h.HabitName,
			Category:       h.Category,
			CompletedCount: int32(h.CompletedCount),
			MissedCount:    int32(h.MissedCount),
			CompletionRate: float32(h.CompletionRate),
		}
	}

	blockerStats := make([]*aicoachservice.BlockerStat, len(stats.blockerStats))
	for i, b := range stats.blockerStats {
		blockerStats[i] = &aicoachservice.BlockerStat{
			Blocker: b.Blocker,
			Count:   int32(b.Count),
		}
	}

	moodStats := make([]*aicoachservice.MoodStat, len(stats.moodStats))
	for i, m := range stats.moodStats {
		moodStats[i] = &aicoachservice.MoodStat{
			Mood:  m.Mood,
			Count: int32(m.Count),
		}
	}

	energyStats := make([]*aicoachservice.EnergyStat, len(stats.energyStats))
	for i, e := range stats.energyStats {
		energyStats[i] = &aicoachservice.EnergyStat{
			Energy: e.Energy,
			Count:  int32(e.Count),
		}
	}

	aiResp, aiErr := l.svcCtx.AICoachRpc.GenerateWeeklyReview(ctx, &aicoachservice.WeeklyReviewRequest{
		UserId:               in.UserId,
		AccountabilityStyle:  accountabilityStyle,
		PreferredTone:        preferredTone,
		DifficultyPreference: difficultyPreference,
		CommonBlockers:       commonBlockers,
		Goals:                goalTitles,
		TotalHabits:          int32(stats.totalHabits),
		CompletionRate:       float32(stats.completionRate),
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
	})

	var aiSummary string
	var suggestedAdjustments []*aicoachservice.WeeklyReviewAdjustment
	var nextWeekPlan *aicoachservice.NextWeekPlan
	if aiErr != nil {
		l.Errorf("AI generation failed, using fallback: %v", aiErr)
		aiSummary = "I couldn't generate a full weekly review right now. Based on your recent activity, focus on maintaining consistency with your core habits and address any recurring blockers you noticed this week."
		suggestedAdjustments = nil
		nextWeekPlan = &aicoachservice.NextWeekPlan{
			Focus:           "Maintain consistency",
			Commitments:     []string{"Complete at least one habit check-in each day"},
			Risks:           []string{"Falling behind on new habits"},
			RecoveryActions: []string{"Scale back to one core habit if overwhelmed"},
		}
	} else {
		aiSummary = aiResp.AiSummary
		suggestedAdjustments = aiResp.SuggestedAdjustments
		nextWeekPlan = aiResp.NextWeekPlan
	}

	moodSummaryJSON, err := json.Marshal(stats.moodMap)
	if err != nil {
		l.Errorf("failed to marshal mood summary: %v", err)
		return nil, status.Error(codes.Internal, "failed to serialize mood summary")
	}
	energySummaryJSON, err := json.Marshal(stats.energyMap)
	if err != nil {
		l.Errorf("failed to marshal energy summary: %v", err)
		return nil, status.Error(codes.Internal, "failed to serialize energy summary")
	}
	habitBreakdownJSON, err := json.Marshal(stats.habitBreakdownsForDB)
	if err != nil {
		l.Errorf("failed to marshal habit breakdown: %v", err)
		return nil, status.Error(codes.Internal, "failed to serialize habit breakdown")
	}
	suggestedAdjustmentsJSON, err := json.Marshal(suggestedAdjustments)
	if err != nil {
		l.Errorf("failed to marshal suggested adjustments: %v", err)
		return nil, status.Error(codes.Internal, "failed to serialize suggested adjustments")
	}
	nextWeekPlanJSON, err := json.Marshal(nextWeekPlan)
	if err != nil {
		l.Errorf("failed to marshal next week plan: %v", err)
		return nil, status.Error(codes.Internal, "failed to serialize next week plan")
	}

	weekStartDate := pgtype.Date{Time: weekStart, Valid: true}

	var completionRate pgtype.Numeric
	if err := completionRate.Scan(fmt.Sprintf("%.2f", stats.completionRate)); err != nil {
		l.Infof("failed to scan completion rate: %v", err)
	}

	var bestDay *string
	if stats.bestDay != "" {
		bestDay = &stats.bestDay
	}
	var hardestDay *string
	if stats.hardestDay != "" {
		hardestDay = &stats.hardestDay
	}
	var topBlocker *string
	if stats.topBlocker != "" {
		topBlocker = &stats.topBlocker
	}
	var aiSummaryPtr *string
	if aiSummary != "" {
		aiSummaryPtr = &aiSummary
	}

	params := db.CreateWeeklyReviewParams{
		UserID:               userID,
		WeekStart:            weekStartDate,
		TotalHabits:          int32(stats.totalHabits),
		CompletedCheckIns:    int32(stats.completedCheckIns),
		MissedCheckIns:       int32(stats.missedCheckIns),
		CompletionRate:       completionRate,
		BestDay:              bestDay,
		HardestDay:           hardestDay,
		TopBlocker:           topBlocker,
		MoodSummary:          moodSummaryJSON,
		EnergySummary:        energySummaryJSON,
		HabitBreakdown:       habitBreakdownJSON,
		AiSummary:            aiSummaryPtr,
		SuggestedAdjustments: suggestedAdjustmentsJSON,
		NextWeekPlan:         nextWeekPlanJSON,
	}

	review, err := l.svcCtx.Repo.WeeklyReviews.CreateWeeklyReview(ctx, params)
	if err != nil {
		l.Errorf("failed to save weekly review: %v", err)
		return nil, status.Error(codes.Internal, "failed to save weekly review")
	}

	// Automatically create plan adjustment suggestions from AI recommendations
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		for _, adjustment := range suggestedAdjustments {
			if adjustment.AdjustmentType == "keep_same" {
				continue // Skip suggestions to keep things the same
			}

			var goalID, habitID uuid.NullUUID
			if adjustment.HabitId != "" {
				if habitUUID, err := uuid.Parse(adjustment.HabitId); err == nil {
					habitID = uuid.NullUUID{UUID: habitUUID, Valid: true}
				}
			}

			metadata := map[string]string{
				"source":          "weekly_review",
				"week_start":      weekStart.Format(time.RFC3339),
				"habit_name":      adjustment.HabitName,
				"adjustment_type": adjustment.AdjustmentType,
				"ai_generated":    "true",
			}
			metadataJSON, _ := json.Marshal(metadata)

			_, err := l.svcCtx.Repo.PlanAdjustmentSuggestions.CreatePlanAdjustmentSuggestion(ctx, db.CreatePlanAdjustmentSuggestionParams{
				UserID:         userID,
				GoalID:         goalID,
				HabitID:        habitID,
				Source:         "weekly_review",
				AdjustmentType: (adjustment.AdjustmentType),
				Reason:         adjustment.Reason,
				Suggestion:     adjustment.Suggestion,
				Metadata:       metadataJSON,
				WeekStart:      pgtype.Date{Time: weekStart, Valid: true},
			})
			if err != nil {
				logx.Errorf("failed to create plan adjustment suggestion: %v", err)
			}
		}
	}()

	return &client.GenerateWeeklyReviewResponse{Review: dbReviewToProto(review)}, nil
}

type weeklyStats struct {
	totalHabits          int
	completedCheckIns    int
	missedCheckIns       int
	completionRate       float64
	bestDay              string
	hardestDay           string
	topBlocker           string
	habitBreakdowns      []prompts.HabitBreakdownInput
	habitBreakdownsForDB []habitBreakdownDB
	blockerStats         []prompts.BlockerInput
	moodStats            []prompts.MoodInput
	energyStats          []prompts.EnergyInput
	moodMap              map[string]int
	energyMap            map[string]int
}

type habitBreakdownDB struct {
	HabitID        string  `json:"habitId"`
	HabitName      string  `json:"habitName"`
	Category       string  `json:"category"`
	TotalCheckIns  int     `json:"totalCheckIns"`
	CompletedCount int     `json:"completedCount"`
	MissedCount    int     `json:"missedCount"`
	CompletionRate float64 `json:"completionRate"`
	LastCheckInAt  string  `json:"lastCheckInAt,omitempty"`
}

func (l *GenerateWeeklyReviewLogic) computeWeeklyStats(userID uuid.UUID, start, end time.Time) (weeklyStats, error) {
	var stats weeklyStats
	stats.moodMap = make(map[string]int)
	stats.energyMap = make(map[string]int)

	habitStats, err := l.svcCtx.Repo.WeeklyReviews.GetCheckInStatsForWeek(l.ctx, userID, start, end)
	if err != nil {
		return stats, err
	}

	stats.totalHabits = len(habitStats)
	stats.habitBreakdowns = make([]prompts.HabitBreakdownInput, 0, len(habitStats))
	stats.habitBreakdownsForDB = make([]habitBreakdownDB, 0, len(habitStats))
	for _, h := range habitStats {
		completed := int(h.CompletedCount)
		missed := int(h.MissedCount)
		total := completed + missed
		var rate float64
		if total > 0 {
			rate = float64(completed) / float64(total) * 100
		}
		stats.completedCheckIns += completed
		stats.missedCheckIns += missed

		stats.habitBreakdowns = append(stats.habitBreakdowns, prompts.HabitBreakdownInput{
			HabitID:        h.HabitID.String(),
			HabitName:      h.HabitName,
			Category:       h.HabitCategory,
			CompletedCount: completed,
			MissedCount:    missed,
			CompletionRate: rate,
		})

		lastCheckInAt := ""
		if t, ok := h.LastCheckInAt.(time.Time); ok {
			lastCheckInAt = t.Format(time.RFC3339)
		}
		stats.habitBreakdownsForDB = append(stats.habitBreakdownsForDB, habitBreakdownDB{
			HabitID:        h.HabitID.String(),
			HabitName:      h.HabitName,
			Category:       h.HabitCategory,
			TotalCheckIns:  total,
			CompletedCount: completed,
			MissedCount:    missed,
			CompletionRate: rate,
			LastCheckInAt:  lastCheckInAt,
		})
	}

	total := stats.completedCheckIns + stats.missedCheckIns
	if total > 0 {
		stats.completionRate = float64(stats.completedCheckIns) / float64(total) * 100
	}

	dailyStats, err := l.svcCtx.Repo.WeeklyReviews.GetDailyCheckInStatsForWeek(l.ctx, userID, start, end)
	if err != nil {
		return stats, err
	}

	var bestRate float64 = -1
	var hardestRate float64 = 101
	for _, d := range dailyStats {
		dayTotal := int(d.TotalCheckIns)
		dayCompleted := int(d.CompletedCount)
		var dayRate float64
		if dayTotal > 0 {
			dayRate = float64(dayCompleted) / float64(dayTotal) * 100
		}
		dayStr := d.Day.Time.Format("Monday")
		if dayRate > bestRate && dayTotal > 0 {
			bestRate = dayRate
			stats.bestDay = dayStr
		}
		if dayRate < hardestRate && dayTotal > 0 {
			hardestRate = dayRate
			stats.hardestDay = dayStr
		}
	}

	blockerStats, err := l.svcCtx.Repo.WeeklyReviews.GetBlockerStatsForWeek(l.ctx, userID, start, end)
	if err != nil {
		return stats, err
	}
	stats.blockerStats = make([]prompts.BlockerInput, 0, len(blockerStats))
	for _, b := range blockerStats {
		stats.blockerStats = append(stats.blockerStats, prompts.BlockerInput{
			Blocker: b.Blocker,
			Count:   int(b.Count),
		})
		if stats.topBlocker == "" {
			stats.topBlocker = b.Blocker
		}
	}

	moodStats, err := l.svcCtx.Repo.WeeklyReviews.GetMoodStatsForWeek(l.ctx, userID, start, end)
	if err != nil {
		return stats, err
	}
	stats.moodStats = make([]prompts.MoodInput, 0, len(moodStats))
	for _, m := range moodStats {
		stats.moodStats = append(stats.moodStats, prompts.MoodInput{
			Mood:  m.Mood,
			Count: int(m.Count),
		})
		stats.moodMap[m.Mood] = int(m.Count)
	}

	energyStats, err := l.svcCtx.Repo.WeeklyReviews.GetEnergyStatsForWeek(l.ctx, userID, start, end)
	if err != nil {
		return stats, err
	}
	stats.energyStats = make([]prompts.EnergyInput, 0, len(energyStats))
	for _, e := range energyStats {
		stats.energyStats = append(stats.energyStats, prompts.EnergyInput{
			Energy: e.Energy,
			Count:  int(e.Count),
		})
		stats.energyMap[e.Energy] = int(e.Count)
	}

	return stats, nil
}

func resolveWeekBounds(weekStartStr string, loc *time.Location) (time.Time, time.Time, error) {
	var weekStart time.Time
	if weekStartStr == "" {
		now := time.Now().In(loc)
		offset := int(time.Monday - now.Weekday())
		if offset > 0 {
			offset -= 7
		}
		weekStart = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc).AddDate(0, 0, offset)
	} else {
		var err error
		weekStart, err = time.ParseInLocation("2006-01-02", weekStartStr, loc)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		// Validate that weekStart is a Monday
		if weekStart.Weekday() != time.Monday {
			return time.Time{}, time.Time{}, fmt.Errorf("weekStart must be a Monday, got %s", weekStart.Weekday())
		}
	}
	weekEnd := weekStart.AddDate(0, 0, 7)
	return weekStart, weekEnd, nil
}

func dbReviewToProto(r db.GetWeeklyReviewRow) *client.WeeklyReview {
	var moodMap map[string]int32
	_ = json.Unmarshal(r.MoodSummary, &moodMap)
	if moodMap == nil {
		moodMap = make(map[string]int32)
	}

	var energyMap map[string]int32
	_ = json.Unmarshal(r.EnergySummary, &energyMap)
	if energyMap == nil {
		energyMap = make(map[string]int32)
	}

	var habitBreakdowns []habitBreakdownDB
	_ = json.Unmarshal(r.HabitBreakdown, &habitBreakdowns)

	protoHabits := make([]*client.WeeklyReviewHabitBreakdown, len(habitBreakdowns))
	for i, h := range habitBreakdowns {
		var lastCheckInAt int64
		if t, err := time.Parse(time.RFC3339, h.LastCheckInAt); err == nil {
			lastCheckInAt = t.Unix()
		}
		protoHabits[i] = &client.WeeklyReviewHabitBreakdown{
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

	var adjustments []prompts.WeeklyReviewAdjustment
	_ = json.Unmarshal(r.SuggestedAdjustments, &adjustments)
	protoAdjustments := make([]*client.WeeklyReviewAdjustment, len(adjustments))
	for i, a := range adjustments {
		protoAdjustments[i] = &client.WeeklyReviewAdjustment{
			HabitId:        a.HabitID,
			HabitName:      a.HabitName,
			Reason:         a.Reason,
			Suggestion:     a.Suggestion,
			AdjustmentType: a.AdjustmentType,
		}
	}

	var nextWeekPlan prompts.WeeklyReviewNextWeekPlan
	_ = json.Unmarshal(r.NextWeekPlan, &nextWeekPlan)
	protoPlan := &client.WeeklyReviewNextWeekPlan{
		Focus:           nextWeekPlan.Focus,
		Commitments:     nextWeekPlan.Commitments,
		Risks:           nextWeekPlan.Risks,
		RecoveryActions: nextWeekPlan.RecoveryActions,
	}

	return &client.WeeklyReview{
		Id:                   r.ID.String(),
		UserId:               r.UserID.String(),
		WeekStart:            r.WeekStart.Time.Format("2006-01-02"),
		WeekEnd:              r.WeekEnd.Time.Format("2006-01-02"),
		TotalHabits:          r.TotalHabits,
		CompletedCheckIns:    r.CompletedCheckIns,
		MissedCheckIns:       r.MissedCheckIns,
		CompletionRate:       parseCompletionRate(r.CompletionRate),
		BestDay:              stringPtrToString(r.BestDay),
		HardestDay:           stringPtrToString(r.HardestDay),
		TopBlocker:           stringPtrToString(r.TopBlocker),
		MoodSummary:          moodMap,
		EnergySummary:        energyMap,
		HabitBreakdown:       protoHabits,
		AiSummary:            stringPtrToString(r.AiSummary),
		SuggestedAdjustments: protoAdjustments,
		NextWeekPlan:         protoPlan,
		GeneratedAt:          r.GeneratedAt.Time.Unix(),
	}
}

func stringPtrToString(s *string) string {
	if s != nil {
		return *s
	}
	return ""
}

func parseCompletionRate(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	var v float64
	_, err := fmt.Sscanf(fmt.Sprintf("%v", n), "%f", &v)
	if err != nil {
		return 0
	}
	return v
}
