package weeklyreviewservicelogic

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/zeromicro/go-zero/core/logx"

	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
)

// Fakes embed the repository interfaces so uncalled methods panic instead of
// silently succeeding.

type fakeWeeklyReviews struct {
	repository.IWeeklyReviews
	habitStats []db.GetCheckInStatsForWeekRow
	dailyStats []db.GetDailyCheckInStatsForWeekRow
	blockers   []db.GetBlockerStatsForWeekRow
	moods      []db.GetMoodStatsForWeekRow
	energies   []db.GetEnergyStatsForWeekRow
}

func (f *fakeWeeklyReviews) GetCheckInStatsForWeek(_ context.Context, _ uuid.UUID, _, _ time.Time) ([]db.GetCheckInStatsForWeekRow, error) {
	return f.habitStats, nil
}

func (f *fakeWeeklyReviews) GetDailyCheckInStatsForWeek(_ context.Context, _ uuid.UUID, _, _ time.Time) ([]db.GetDailyCheckInStatsForWeekRow, error) {
	return f.dailyStats, nil
}

func (f *fakeWeeklyReviews) GetBlockerStatsForWeek(_ context.Context, _ uuid.UUID, _, _ time.Time) ([]db.GetBlockerStatsForWeekRow, error) {
	return f.blockers, nil
}

func (f *fakeWeeklyReviews) GetMoodStatsForWeek(_ context.Context, _ uuid.UUID, _, _ time.Time) ([]db.GetMoodStatsForWeekRow, error) {
	return f.moods, nil
}

func (f *fakeWeeklyReviews) GetEnergyStatsForWeek(_ context.Context, _ uuid.UUID, _, _ time.Time) ([]db.GetEnergyStatsForWeekRow, error) {
	return f.energies, nil
}

type fakeCategories struct {
	repository.ICategories
	cats []db.Category
}

func (f *fakeCategories) GetCategoriesByIDs(_ context.Context, _ []uuid.UUID) ([]db.Category, error) {
	return f.cats, nil
}

func newStatsLogic(reviews *fakeWeeklyReviews, cats *fakeCategories) *weeklyStatsLogic {
	return &weeklyStatsLogic{
		svcCtx: &svc.ServiceContext{
			Repo: &repository.Repository{
				WeeklyReviews: reviews,
				Categories:    cats,
			},
		},
		Logger: logx.WithContext(context.Background()),
	}
}

// A past week fully in the past so weekEndCapped == weekEnd.
func pastWeek() (time.Time, time.Time) {
	start := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC) // Monday
	return start, start.AddDate(0, 0, 7)
}

func dayRow(day time.Time, completed, missed int64) db.GetDailyCheckInStatsForWeekRow {
	return db.GetDailyCheckInStatsForWeekRow{
		Day:            pgtype.Date{Time: day, Valid: true},
		CompletedCount: completed,
		MissedCount:    missed,
	}
}

func TestComputeWeeklyStats_MissingDaysCountAsMissed(t *testing.T) {
	weekStart, weekEnd := pastWeek()
	userID := uuid.New()
	habitID := uuid.New()

	// Habit existed all week; only 3 check-ins logged → the other 4 days are
	// "missing" and must count against the completion rate.
	reviews := &fakeWeeklyReviews{
		habitStats: []db.GetCheckInStatsForWeekRow{{
			HabitID:        habitID,
			HabitName:      "Meditate",
			HabitCreatedAt: pgtype.Timestamptz{Time: weekStart, Valid: true},
			CompletedCount: 2,
			MissedCount:    1,
		}},
		dailyStats: []db.GetDailyCheckInStatsForWeekRow{
			dayRow(weekStart, 1, 0),
			dayRow(weekStart.AddDate(0, 0, 1), 1, 0),
			dayRow(weekStart.AddDate(0, 0, 2), 0, 1),
		},
	}

	stats, err := newStatsLogic(reviews, &fakeCategories{}).computeWeeklyStats(context.Background(), userID, weekStart, weekEnd)
	require.NoError(t, err)

	assert.Equal(t, 1, stats.totalHabits)
	assert.Equal(t, 2, stats.completedCheckIns)
	// 1 explicit miss + 4 missing days = 5 missed.
	assert.Equal(t, 5, stats.missedCheckIns)
	assert.InDelta(t, 2.0/7.0*100, stats.completionRate, 0.01)

	require.Len(t, stats.habitBreakdownsForDB, 1)
	assert.Equal(t, 7, stats.habitBreakdownsForDB[0].TotalCheckIns)
	assert.Equal(t, 5, stats.habitBreakdownsForDB[0].MissedCount)
}

func TestComputeWeeklyStats_HabitCreatedMidWeek(t *testing.T) {
	weekStart, weekEnd := pastWeek()
	userID := uuid.New()

	// Habit created Thursday — only Thu..Sun are expected days (4 days).
	created := weekStart.AddDate(0, 0, 3)
	reviews := &fakeWeeklyReviews{
		habitStats: []db.GetCheckInStatsForWeekRow{{
			HabitID:        uuid.New(),
			HabitName:      "Read",
			HabitCreatedAt: pgtype.Timestamptz{Time: created, Valid: true},
			CompletedCount: 3,
			MissedCount:    0,
		}},
		dailyStats: []db.GetDailyCheckInStatsForWeekRow{
			dayRow(created, 1, 0),
			dayRow(created.AddDate(0, 0, 1), 1, 0),
			dayRow(created.AddDate(0, 0, 2), 1, 0),
		},
	}

	stats, err := newStatsLogic(reviews, &fakeCategories{}).computeWeeklyStats(context.Background(), userID, weekStart, weekEnd)
	require.NoError(t, err)

	require.Len(t, stats.habitBreakdownsForDB, 1)
	// expected = Thu..Sun = 4 days; 3 completed → 1 missing → missed=1.
	assert.Equal(t, 4, stats.habitBreakdownsForDB[0].TotalCheckIns)
	assert.Equal(t, 1, stats.habitBreakdownsForDB[0].MissedCount)

	// Days before the habit existed must report Expected=0.
	for _, d := range stats.dailyCoverage {
		if d.Date < created.Format("2006-01-02") {
			assert.Equal(t, 0, d.Expected, "day %s predates habit creation", d.Date)
		}
	}
}

func TestComputeWeeklyStats_BestAndHardestDay(t *testing.T) {
	weekStart, weekEnd := pastWeek()
	userID := uuid.New()

	reviews := &fakeWeeklyReviews{
		habitStats: []db.GetCheckInStatsForWeekRow{{
			HabitID:        uuid.New(),
			HabitName:      "Run",
			HabitCreatedAt: pgtype.Timestamptz{Time: weekStart, Valid: true},
		}},
		dailyStats: []db.GetDailyCheckInStatsForWeekRow{
			dayRow(weekStart, 1, 0),                  // Monday 100%
			dayRow(weekStart.AddDate(0, 0, 3), 0, 0), // Thursday 0% (expected=1)
		},
	}

	stats, err := newStatsLogic(reviews, &fakeCategories{}).computeWeeklyStats(context.Background(), userID, weekStart, weekEnd)
	require.NoError(t, err)
	assert.Equal(t, "Monday", stats.bestDay)
	assert.NotEmpty(t, stats.hardestDay)
	assert.NotEqual(t, stats.bestDay, stats.hardestDay)
}

func TestComputeWeeklyStats_SingleActiveDayDropsHardest(t *testing.T) {
	weekStart, weekEnd := pastWeek()
	userID := uuid.New()

	// Only one day has any expectation recorded — best == hardest must
	// collapse to just bestDay (a single day can't be both best and worst).
	reviews := &fakeWeeklyReviews{
		habitStats: []db.GetCheckInStatsForWeekRow{{
			HabitID:        uuid.New(),
			HabitName:      "Run",
			HabitCreatedAt: pgtype.Timestamptz{Time: weekStart, Valid: true},
			CompletedCount: 1,
		}},
		dailyStats: []db.GetDailyCheckInStatsForWeekRow{
			dayRow(weekStart, 1, 0),
		},
	}

	stats, err := newStatsLogic(reviews, &fakeCategories{}).computeWeeklyStats(context.Background(), userID, weekStart, weekEnd)
	require.NoError(t, err)
	// The one fully-completed day is best; all other days are 0% so hardest is
	// a distinct day — but when all days tie, hardest must be empty.
	if stats.hardestDay == stats.bestDay {
		t.Fatalf("hardestDay must never equal bestDay (got %q)", stats.hardestDay)
	}
}

func TestComputeWeeklyStats_TopBlockerAndMoodEnergyMaps(t *testing.T) {
	weekStart, weekEnd := pastWeek()
	userID := uuid.New()

	reviews := &fakeWeeklyReviews{
		blockers: []db.GetBlockerStatsForWeekRow{
			{Blocker: "tired", Count: 4},
			{Blocker: "busy", Count: 2},
		},
		moods: []db.GetMoodStatsForWeekRow{
			{Mood: "good", Count: 3},
			{Mood: "low", Count: 1},
		},
		energies: []db.GetEnergyStatsForWeekRow{
			{Energy: "high", Count: 2},
		},
	}

	stats, err := newStatsLogic(reviews, &fakeCategories{}).computeWeeklyStats(context.Background(), userID, weekStart, weekEnd)
	require.NoError(t, err)
	assert.Equal(t, "tired", stats.topBlocker)
	assert.Equal(t, 3, stats.moodMap["good"])
	assert.Equal(t, 2, stats.energyMap["high"])
	require.Len(t, stats.blockerStats, 2)
	assert.Equal(t, "tired", stats.blockerStats[0].Blocker)
}

func TestResolveWeekBounds(t *testing.T) {
	loc := time.UTC

	t.Run("explicit Monday accepted", func(t *testing.T) {
		start, end, err := resolveWeekBounds("2026-09-07", loc)
		require.NoError(t, err)
		assert.Equal(t, time.Monday, start.Weekday())
		assert.Equal(t, 7*24*time.Hour, end.Sub(start))
	})

	t.Run("non-Monday rejected", func(t *testing.T) {
		_, _, err := resolveWeekBounds("2026-09-08", loc) // Tuesday
		require.Error(t, err)
	})

	t.Run("invalid date rejected", func(t *testing.T) {
		_, _, err := resolveWeekBounds("not-a-date", loc)
		require.Error(t, err)
	})

	t.Run("empty defaults to current week Monday", func(t *testing.T) {
		start, end, err := resolveWeekBounds("", loc)
		require.NoError(t, err)
		assert.Equal(t, time.Monday, start.Weekday())
		assert.True(t, end.After(start))
	})
}
