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

func (l *weeklyStatsLogic) computeWeeklyStats(ctx context.Context, userID uuid.UUID, start, end time.Time) (weeklyStats, error) {
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

	// Unmarshal adjustments using aicoachservice types for backward
	// compatibility with existing DB rows (snake_case JSON keys).
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
