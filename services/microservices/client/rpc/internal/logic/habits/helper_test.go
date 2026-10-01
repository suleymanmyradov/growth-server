package habitslogic

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

// dateRow builds a ListHabitHistoryRow whose local_date is the given calendar
// day. pgtype.Date decodes DATE columns at UTC midnight.
func dateRow(habitID uuid.UUID, year int, month time.Month, day int) db.ListHabitHistoryRow {
	return db.ListHabitHistoryRow{
		HabitID:   habitID,
		LocalDate: pgtype.Date{Time: time.Date(year, month, day, 0, 0, 0, 0, time.UTC), Valid: true},
	}
}

func countTrue(days []bool) int {
	n := 0
	for _, d := range days {
		if d {
			n++
		}
	}
	return n
}

func TestBuildRecentHistory(t *testing.T) {
	habitID := uuid.New()
	otherHabit := uuid.New()

	// 2026-09-29 22:00 in New York = 2026-09-30 02:00 UTC. "Today" must be the
	// user's calendar day (Sep 29), not the UTC day.
	ny, err := time.LoadLocation("America/New_York")
	assert.NoError(t, err)
	tokyo, err := time.LoadLocation("Asia/Tokyo")
	assert.NoError(t, err)

	t.Run("west of UTC: today lands in the last bucket, not shifted", func(t *testing.T) {
		today := time.Date(2026, 9, 29, 22, 0, 0, 0, ny)
		rows := []db.ListHabitHistoryRow{
			dateRow(habitID, 2026, 9, 29), // today
			dateRow(habitID, 2026, 9, 28), // yesterday
			dateRow(habitID, 2026, 9, 2),  // window start (27 days ago)
		}
		got := buildRecentHistory(habitID, today, rows)
		assert.Len(t, got, 28)
		assert.True(t, got[27], "today must be the last bucket")
		assert.True(t, got[26], "yesterday")
		assert.True(t, got[0], "window start")
		assert.Equal(t, 3, countTrue(got))
	})

	t.Run("east of UTC: buckets are correct", func(t *testing.T) {
		today := time.Date(2026, 9, 30, 1, 0, 0, 0, tokyo)
		rows := []db.ListHabitHistoryRow{
			dateRow(habitID, 2026, 9, 30), // today
			dateRow(habitID, 2026, 9, 3),  // window start
		}
		got := buildRecentHistory(habitID, today, rows)
		assert.True(t, got[27])
		assert.True(t, got[0])
		assert.Equal(t, 2, countTrue(got))
	})

	t.Run("UTC baseline", func(t *testing.T) {
		today := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
		rows := []db.ListHabitHistoryRow{
			dateRow(habitID, 2026, 9, 29),
			dateRow(habitID, 2026, 9, 1), // one day before the window — excluded
		}
		got := buildRecentHistory(habitID, today, rows)
		assert.True(t, got[27])
		assert.Equal(t, 1, countTrue(got))
	})

	t.Run("filters other habits and invalid dates", func(t *testing.T) {
		today := time.Date(2026, 9, 29, 12, 0, 0, 0, time.UTC)
		rows := []db.ListHabitHistoryRow{
			dateRow(otherHabit, 2026, 9, 29),
			{HabitID: habitID, LocalDate: pgtype.Date{Valid: false}},
		}
		got := buildRecentHistory(habitID, today, rows)
		assert.Equal(t, 0, countTrue(got))
	})
}
