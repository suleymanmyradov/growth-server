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
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/client/aicoachservice"
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

// Page sizes for paging the full set of inputs that feed pattern detection
// and the AI prompt. These bound the per-query result set, not the total
// amount of data considered (fetchAllPages keeps going until exhausted).
const (
	checkInPageSize int32 = 500
	habitPageSize   int32 = 200
	goalPageSize    int32 = 100
)

// fetchAllPages repeatedly calls fetch with an increasing offset until a short
// (final) page is returned, accumulating every row. It exists so weekly-review
// inputs are computed over the user's full data instead of being silently
// truncated at a hardcoded cap. On error it returns the rows gathered so far
// alongside the error, letting callers decide whether a partial set is usable.
func fetchAllPages[T any](pageSize int32, fetch func(limit, offset int32) ([]T, error)) ([]T, error) {
	var all []T
	for offset := int32(0); ; offset += pageSize {
		page, err := fetch(pageSize, offset)
		if err != nil {
			return all, err
		}
		all = append(all, page...)
		if int32(len(page)) < pageSize {
			return all, nil
		}
	}
}

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

	// Dedupe concurrent generations for the same (user, week). A double-submit
	// (retry, two devices) would otherwise both pass the "no existing review"
	// check below and each run the expensive AI pipeline, then clobber each
	// other's row. The DB unique upsert already prevents duplicate rows; this
	// collapses the redundant work so only one generation runs and the others
	// share its result.
	sfKey := userID.String() + ":" + weekStart.Format("2006-01-02")
	res, err, _ := l.svcCtx.WeeklyReviewSF.Do(sfKey, func() (any, error) {
		return l.generateWeeklyReview(ctx, in, userID, settings, loc, weekStart, weekEnd)
	})
	if err != nil {
		return nil, err
	}
	return res.(*client.GenerateWeeklyReviewResponse), nil
}

func (l *GenerateWeeklyReviewLogic) generateWeeklyReview(ctx context.Context, in *client.GenerateWeeklyReviewRequest, userID uuid.UUID, settings db.UserSetting, loc *time.Location, weekStart, weekEnd time.Time) (*client.GenerateWeeklyReviewResponse, error) {
	if !in.ForceRegenerate {
		existing, err := l.svcCtx.Repo.WeeklyReviews.GetWeeklyReview(ctx, userID, weekStart)
		if err == nil && existing.ID != uuid.Nil {
			return &client.GenerateWeeklyReviewResponse{Review: dbReviewToProto(existing)}, nil
		}
	} else {
		// Cooldown for forced regeneration, configurable via YAML (default 1h).
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

	stats, err := l.computeWeeklyStats(ctx, userID, weekStart, weekEnd)
	if err != nil {
		l.Errorf("compute weekly stats: %v", err)
		return nil, status.Error(codes.Internal, "failed to compute weekly stats")
	}

	// Get raw check-ins and habits for pattern detection. These feed the
	// pattern engine and AI prompt, so they are paged in full rather than
	// truncated at a fixed cap, which would silently skew the analysis for
	// users with many habits or check-ins. On a partial failure we keep
	// whatever pages were fetched (better a partial signal than none).
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

	// Analyze patterns using the pattern detection service
	patternInsights := l.svcCtx.PatternDetection.AnalyzeFullFromData(weekCheckIns, weekHabits, streakByHabit, loc)

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
		// The LLM occasionally omits or invents an adjustment type; coerce to a
		// valid enum value before persisting so stored data honors the contract.
		for _, adj := range suggestedAdjustments {
			if adj != nil {
				adj.AdjustmentType = normalizeAdjustmentType(adj.AdjustmentType)
			}
		}
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

	// Log activity
	weekLabel := weekStart.Format("Jan 2, 2006")
	desc := fmt.Sprintf("Generated weekly review for %s", weekLabel)
	meta, _ := json.Marshal(map[string]string{
		"reviewId":  review.ID.String(),
		"weekStart": weekStart.Format("2006-01-02"),
	})
	if _, err := l.svcCtx.Repo.Activities.CreateActivity(ctx, db.CreateActivityParams{
		Type:        "weekly_review_generated",
		Title:       fmt.Sprintf("Weekly review for %s", weekLabel),
		Description: &desc,
		Metadata:    meta,
		UserID:      userID,
	}); err != nil {
		l.Errorf("Failed to log weekly_review_generated activity: %v", err)
	}

	// Automatically create plan adjustment suggestions from AI recommendations
	go func() {
		defer func() {
			if r := recover(); r != nil {
				logx.Errorf("panic while creating plan adjustment suggestions: %v", r)
			}
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		for _, adjustment := range suggestedAdjustments {
			if adjustment == nil {
				continue
			}
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

func (l *GenerateWeeklyReviewLogic) computeWeeklyStats(ctx context.Context, userID uuid.UUID, start, end time.Time) (weeklyStats, error) {
	var stats weeklyStats
	stats.moodMap = make(map[string]int)
	stats.energyMap = make(map[string]int)

	habitStats, err := l.svcCtx.Repo.WeeklyReviews.GetCheckInStatsForWeek(ctx, userID, start, end)
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

	dailyStats, err := l.svcCtx.Repo.WeeklyReviews.GetDailyCheckInStatsForWeek(ctx, userID, start, end)
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

	// With a single active day (or all days at the same rate) the best and
	// hardest day resolve to the same value, which is meaningless to report.
	// Drop hardestDay so the review only highlights a genuinely distinct day.
	if stats.hardestDay == stats.bestDay {
		stats.hardestDay = ""
	}

	blockerStats, err := l.svcCtx.Repo.WeeklyReviews.GetBlockerStatsForWeek(ctx, userID, start, end)
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

	moodStats, err := l.svcCtx.Repo.WeeklyReviews.GetMoodStatsForWeek(ctx, userID, start, end)
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

	energyStats, err := l.svcCtx.Repo.WeeklyReviews.GetEnergyStatsForWeek(ctx, userID, start, end)
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

	var adjustments []*aicoachservice.WeeklyReviewAdjustment
	_ = json.Unmarshal(r.SuggestedAdjustments, &adjustments)
	protoAdjustments := make([]*client.WeeklyReviewAdjustment, 0, len(adjustments))
	for _, a := range adjustments {
		if a == nil {
			continue
		}
		protoAdjustments = append(protoAdjustments, &client.WeeklyReviewAdjustment{
			HabitId:        a.HabitId,
			HabitName:      a.HabitName,
			Reason:         a.Reason,
			Suggestion:     a.Suggestion,
			AdjustmentType: normalizeAdjustmentType(a.AdjustmentType),
		})
	}

	var nextWeekPlan aicoachservice.NextWeekPlan
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

// normalizeAdjustmentType coerces an AI-provided adjustment type to a valid
// enum value. The LLM occasionally returns an empty or unexpected value;
// callers (and the API contract) require one of the known types, so anything
// invalid defaults to "keep_same".
func normalizeAdjustmentType(t string) string {
	switch t {
	case "reduce_difficulty", "change_time", "clarify_plan", "keep_same", "pause_habit":
		return t
	default:
		return "keep_same"
	}
}

func parseCompletionRate(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil {
		return 0
	}
	return f.Float64
}
