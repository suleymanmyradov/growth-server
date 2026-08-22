package weeklyreviewservicelogic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/pkg/prompts"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"

	"github.com/zeromicro/go-zero/core/logx"
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

// weeklyStatsLogic holds the dependencies for computeWeeklyStats. It is
// embedded by PrepareWeeklyReviewLogic.
type weeklyStatsLogic struct {
	svcCtx *svc.ServiceContext
	logx.Logger
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
	dailyCoverage        []dailyCoverageEntry
}

// dailyCoverageEntry represents one day's check-in activity within the week.
// expectedCheckIns = number of habits that existed on that day (created before
// or on that day). missingCheckIns = expected - (completed + missed + skipped).
// A day with no check-in rows at all will have completed=0, missed=0, and
// missing=expected, making the gap visible to the coach.
type dailyCoverageEntry struct {
	Date           string // "2006-01-02"
	DayName        string // "Monday"
	Expected       int
	Completed      int
	Missed         int
	Missing        int // expected but no check-in row was created
	CompletionRate float64
}

type habitBreakdownDB struct {
	HabitID        string  `json:"habit_id"`
	HabitName      string  `json:"habit_name"`
	Category       string  `json:"category"`
	TotalCheckIns  int     `json:"total_check_ins"`
	CompletedCount int     `json:"completed_count"`
	MissedCount    int     `json:"missed_count"`
	CompletionRate float64 `json:"completion_rate"`
	LastCheckInAt  string  `json:"last_check_in_at,omitempty"`
}

// weeklyReviewAdjustmentDB is the DB-level JSON shape for the
// suggested_adjustments column. Snake_case per DB convention.
type weeklyReviewAdjustmentDB struct {
	HabitID        string `json:"habit_id"`
	HabitName      string `json:"habit_name"`
	Reason         string `json:"reason"`
	Suggestion     string `json:"suggestion"`
	AdjustmentType string `json:"adjustment_type"`
}

// nextWeekPlanDB is the DB-level JSON shape for the next_week_plan column.
// Snake_case per DB convention.
type nextWeekPlanDB struct {
	Focus           string   `json:"focus"`
	Commitments     []string `json:"commitments"`
	Risks           []string `json:"risks"`
	RecoveryActions []string `json:"recovery_actions"`
}

// habitBreakdownDBLegacy mirrors the old camelCase JSON keys used before
// the snake_case DB convention. Only used for backward-compatible reads.
type habitBreakdownDBLegacy struct {
	HabitID        string  `json:"habitId"`
	HabitName      string  `json:"habitName"`
	Category       string  `json:"category"`
	TotalCheckIns  int     `json:"totalCheckIns"`
	CompletedCount int     `json:"completedCount"`
	MissedCount    int     `json:"missedCount"`
	CompletionRate float64 `json:"completionRate"`
	LastCheckInAt  string  `json:"lastCheckInAt,omitempty"`
}

func (l *weeklyStatsLogic) computeWeeklyStats(ctx context.Context, userID uuid.UUID, start, end time.Time) (weeklyStats, error) {
	var stats weeklyStats
	stats.moodMap = make(map[string]int)
	stats.energyMap = make(map[string]int)

	habitStats, err := l.svcCtx.Repo.WeeklyReviews.GetCheckInStatsForWeek(ctx, userID, start, end)
	if err != nil {
		return stats, err
	}

	// Resolve category names via the Categories interface (no cross-subservice JOIN).
	categoryIDs := make([]uuid.UUID, 0, len(habitStats))
	for _, h := range habitStats {
		if h.HabitCategoryID.Valid {
			categoryIDs = append(categoryIDs, h.HabitCategoryID.UUID)
		}
	}
	categoryMap := make(map[uuid.UUID]string)
	if len(categoryIDs) > 0 {
		cats, err := l.svcCtx.Repo.Categories.GetCategoriesByIDs(ctx, categoryIDs)
		if err == nil {
			for _, c := range cats {
				categoryMap[c.ID] = c.Slug
			}
		}
	}

	// Cap the "expected" window at today (in UTC) so we don't count future
	// days as missed. For past weeks, now is after weekEnd so the full week
	// is counted.
	now := time.Now().UTC()
	weekEndCapped := end
	if now.Before(weekEndCapped) {
		weekEndCapped = now
	}

	stats.totalHabits = len(habitStats)
	stats.habitBreakdowns = make([]prompts.HabitBreakdownInput, 0, len(habitStats))
	stats.habitBreakdownsForDB = make([]habitBreakdownDB, 0, len(habitStats))
	for _, h := range habitStats {
		completed := int(h.CompletedCount)
		missed := int(h.MissedCount)

		// Compute expected check-in days for this habit: from the later of
		// (habit creation date, week start) to the earlier of (now, week end).
		// Days with no check-in row at all are "missing" — they should count
		// as missed so the completion rate reflects actual engagement, not
		// just the days the user happened to open the app.
		habitStart := start
		if h.HabitCreatedAt.Valid {
			createdDate := h.HabitCreatedAt.Time.In(start.Location()).Truncate(24 * time.Hour)
			if createdDate.After(habitStart) {
				habitStart = createdDate
			}
		}
		expectedDays := 0
		if weekEndCapped.After(habitStart) {
			expectedDays = int(weekEndCapped.Sub(habitStart).Hours() / 24)
			// Round up partial days — if the habit was created at noon, the
			// creation day still counts as an expected day.
			if weekEndCapped.Sub(habitStart).Hours()/24 > float64(expectedDays) {
				expectedDays++
			}
		}
		// Cap at 7 (week length) to avoid off-by-one from timezone rounding.
		if expectedDays > 7 {
			expectedDays = 7
		}
		if expectedDays < 0 {
			expectedDays = 0
		}

		missing := expectedDays - completed - missed
		if missing < 0 {
			missing = 0
		}
		// Treat missing days as missed so they lower the completion rate.
		missed += missing
		total := completed + missed
		var rate float64
		if total > 0 {
			rate = float64(completed) / float64(total) * 100
		}
		stats.completedCheckIns += completed
		stats.missedCheckIns += missed

		category := ""
		if h.HabitCategoryID.Valid {
			category = categoryMap[h.HabitCategoryID.UUID]
		}

		stats.habitBreakdowns = append(stats.habitBreakdowns, prompts.HabitBreakdownInput{
			HabitID:        h.HabitID.String(),
			HabitName:      h.HabitName,
			Category:       category,
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
			Category:       category,
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

	// Build a daily coverage map: for each day in the week, show expected vs
	// actual check-ins. Days with no check-in rows at all are explicitly
	// included so the coach can see gaps and describe temporal patterns
	// (e.g. "you started strong on Monday but trailed off by Thursday").
	dailyByDate := make(map[string]db.GetDailyCheckInStatsForWeekRow, len(dailyStats))
	for _, d := range dailyStats {
		dailyByDate[d.Day.Time.Format("2006-01-02")] = d
	}
	stats.dailyCoverage = make([]dailyCoverageEntry, 0, 7)
	for day := start; day.Before(end); day = day.AddDate(0, 0, 1) {
		dayKey := day.Format("2006-01-02")
		if day.After(weekEndCapped) || dayKey == weekEndCapped.Format("2006-01-02") {
			// Don't include today (it's not over yet) or future days.
			if !day.Before(weekEndCapped) {
				break
			}
		}
		// Count how many habits were expected on this day.
		dayExpected := 0
		for _, h := range habitStats {
			if h.HabitCreatedAt.Valid {
				createdDate := h.HabitCreatedAt.Time.In(start.Location()).Truncate(24 * time.Hour)
				if createdDate.After(day) {
					continue
				}
			}
			dayExpected++
		}
		dayCompleted := 0
		dayMissed := 0
		if d, ok := dailyByDate[dayKey]; ok {
			dayCompleted = int(d.CompletedCount)
			dayMissed = int(d.MissedCount)
		}
		dayMissing := dayExpected - dayCompleted - dayMissed
		if dayMissing < 0 {
			dayMissing = 0
		}
		dayRate := 0.0
		if dayExpected > 0 {
			dayRate = float64(dayCompleted) / float64(dayExpected) * 100
		}
		stats.dailyCoverage = append(stats.dailyCoverage, dailyCoverageEntry{
			Date:           dayKey,
			DayName:        day.Format("Monday"),
			Expected:       dayExpected,
			Completed:      dayCompleted,
			Missed:         dayMissed,
			Missing:        dayMissing,
			CompletionRate: dayRate,
		})
	}

	var bestRate float64 = -1
	var hardestRate float64 = 101
	for _, d := range stats.dailyCoverage {
		if d.Expected == 0 {
			continue
		}
		if d.CompletionRate > bestRate {
			bestRate = d.CompletionRate
			stats.bestDay = d.DayName
		}
		if d.CompletionRate < hardestRate {
			hardestRate = d.CompletionRate
			stats.hardestDay = d.DayName
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
	// Detect legacy camelCase JSON rows (written before the snake_case DB
	// convention) by checking for the old "habitId" key in the raw bytes.
	if len(r.HabitBreakdown) > 0 && bytes.Contains(r.HabitBreakdown, []byte(`"habitId"`)) {
		var legacy []habitBreakdownDBLegacy
		_ = json.Unmarshal(r.HabitBreakdown, &legacy)
		habitBreakdowns = make([]habitBreakdownDB, len(legacy))
		for i, h := range legacy {
			habitBreakdowns[i] = habitBreakdownDB(h)
		}
	} else {
		_ = json.Unmarshal(r.HabitBreakdown, &habitBreakdowns)
	}

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

	// Unmarshal adjustments (snake_case DB JSON keys).
	var adjustments []weeklyReviewAdjustmentDB
	_ = json.Unmarshal(r.SuggestedAdjustments, &adjustments)
	protoAdjustments := make([]*client.WeeklyReviewAdjustment, 0, len(adjustments))
	for _, a := range adjustments {
		protoAdjustments = append(protoAdjustments, &client.WeeklyReviewAdjustment{
			HabitId:        a.HabitID,
			HabitName:      a.HabitName,
			Reason:         a.Reason,
			Suggestion:     a.Suggestion,
			AdjustmentType: normalizeAdjustmentType(a.AdjustmentType),
		})
	}

	var nextWeekPlan nextWeekPlanDB
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
