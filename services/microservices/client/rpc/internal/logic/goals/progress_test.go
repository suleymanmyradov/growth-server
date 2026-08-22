package goalslogic

import (
	"strconv"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

func numFloat(f float64) pgtype.Numeric {
	n := pgtype.Numeric{}
	_ = n.Scan(strconv.FormatFloat(f, 'f', -1, 64))
	return n
}

func numInvalid() pgtype.Numeric {
	return pgtype.Numeric{Valid: false}
}

func ts(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

func TestComputeProgress(t *testing.T) {
	cases := []struct {
		name  string
		goal  db.GetGoalRow
		input ProgressInputs
		want  int32
	}{
		// ── manual (passthrough) ────────────────────────────────────────────
		{"manual passthrough", db.GetGoalRow{Measurement: MeasurementManual, Progress: 42}, ProgressInputs{}, 42},
		{"manual zero", db.GetGoalRow{Measurement: MeasurementManual, Progress: 0}, ProgressInputs{}, 0},
		{"manual full", db.GetGoalRow{Measurement: MeasurementManual, Progress: 100}, ProgressInputs{}, 100},

		// ── binary ──────────────────────────────────────────────────────────
		{"binary not done", db.GetGoalRow{Measurement: MeasurementBinary, Completed: false}, ProgressInputs{}, 0},
		{"binary done", db.GetGoalRow{Measurement: MeasurementBinary, Completed: true}, ProgressInputs{}, 100},

		// ── numeric (ascending: start 0, target 100) ────────────────────────
		{"numeric empty", db.GetGoalRow{Measurement: MeasurementNumeric, StartValue: numFloat(0), CurrentValue: numFloat(0), TargetValue: numFloat(100)}, ProgressInputs{}, 0},
		{"numeric partial", db.GetGoalRow{Measurement: MeasurementNumeric, StartValue: numFloat(0), CurrentValue: numFloat(50), TargetValue: numFloat(100)}, ProgressInputs{}, 50},
		{"numeric exact target", db.GetGoalRow{Measurement: MeasurementNumeric, StartValue: numFloat(0), CurrentValue: numFloat(100), TargetValue: numFloat(100)}, ProgressInputs{}, 100},
		{"numeric overshoot clamped", db.GetGoalRow{Measurement: MeasurementNumeric, StartValue: numFloat(0), CurrentValue: numFloat(150), TargetValue: numFloat(100)}, ProgressInputs{}, 100},
		{"numeric negative clamped", db.GetGoalRow{Measurement: MeasurementNumeric, StartValue: numFloat(0), CurrentValue: numFloat(-10), TargetValue: numFloat(100)}, ProgressInputs{}, 0},

		// ── numeric (descending: weight loss start 80, target 72) ───────────
		{"numeric descending partial", db.GetGoalRow{Measurement: MeasurementNumeric, StartValue: numFloat(80), CurrentValue: numFloat(76), TargetValue: numFloat(72)}, ProgressInputs{}, 50},
		{"numeric descending done", db.GetGoalRow{Measurement: MeasurementNumeric, StartValue: numFloat(80), CurrentValue: numFloat(72), TargetValue: numFloat(72)}, ProgressInputs{}, 100},
		{"numeric descending overshoot clamped", db.GetGoalRow{Measurement: MeasurementNumeric, StartValue: numFloat(80), CurrentValue: numFloat(68), TargetValue: numFloat(72)}, ProgressInputs{}, 100},

		// ── numeric edge: start == target (guard, DB CHECK prevents this) ───
		{"numeric start==target guard", db.GetGoalRow{Measurement: MeasurementNumeric, StartValue: numFloat(50), CurrentValue: numFloat(50), TargetValue: numFloat(50)}, ProgressInputs{}, 0},

		// ── numeric invalid values default to 0 ─────────────────────────────
		{"numeric invalid target", db.GetGoalRow{Measurement: MeasurementNumeric, StartValue: numFloat(0), CurrentValue: numFloat(50), TargetValue: numInvalid()}, ProgressInputs{}, 0},

		// ── milestone ───────────────────────────────────────────────────────
		{"milestone zero steps", db.GetGoalRow{Measurement: MeasurementMilestone}, ProgressInputs{MilestoneTotal: 0, MilestoneDone: 0}, 0},
		{"milestone none done", db.GetGoalRow{Measurement: MeasurementMilestone}, ProgressInputs{MilestoneTotal: 5, MilestoneDone: 0}, 0},
		{"milestone partial", db.GetGoalRow{Measurement: MeasurementMilestone}, ProgressInputs{MilestoneTotal: 4, MilestoneDone: 1}, 25},
		{"milestone all done", db.GetGoalRow{Measurement: MeasurementMilestone}, ProgressInputs{MilestoneTotal: 3, MilestoneDone: 3}, 100},
		{"milestone overshoot clamped", db.GetGoalRow{Measurement: MeasurementMilestone}, ProgressInputs{MilestoneTotal: 3, MilestoneDone: 5}, 100},
		{"milestone rounding", db.GetGoalRow{Measurement: MeasurementMilestone}, ProgressInputs{MilestoneTotal: 3, MilestoneDone: 2}, 67}, // 66.67 → 67

		// ── habit ───────────────────────────────────────────────────────────
		{"habit no habits linked", db.GetGoalRow{Measurement: MeasurementHabit}, ProgressInputs{HabitExpectedDays: 0, HabitCompletedDays: 0}, 0},
		{"habit none completed", db.GetGoalRow{Measurement: MeasurementHabit}, ProgressInputs{HabitExpectedDays: 30, HabitCompletedDays: 0}, 0},
		{"habit partial", db.GetGoalRow{Measurement: MeasurementHabit}, ProgressInputs{HabitExpectedDays: 10, HabitCompletedDays: 3}, 30},
		{"habit full", db.GetGoalRow{Measurement: MeasurementHabit}, ProgressInputs{HabitExpectedDays: 7, HabitCompletedDays: 7}, 100},
		{"habit overshoot clamped", db.GetGoalRow{Measurement: MeasurementHabit}, ProgressInputs{HabitExpectedDays: 5, HabitCompletedDays: 10}, 100},
		{"habit rounding", db.GetGoalRow{Measurement: MeasurementHabit}, ProgressInputs{HabitExpectedDays: 3, HabitCompletedDays: 2}, 67}, // 66.67 → 67
		{"habit zero expected guard", db.GetGoalRow{Measurement: MeasurementHabit}, ProgressInputs{HabitExpectedDays: 0, HabitCompletedDays: 5}, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ComputeProgress(tc.goal, tc.input)
			if got != tc.want {
				t.Fatalf("ComputeProgress = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestHabitWindow(t *testing.T) {
	now := time.Date(2026, 8, 9, 12, 30, 0, 0, time.UTC)
	nowDate := truncateToDate(now) // 2026-08-09 00:00
	created := time.Date(2026, 7, 1, 8, 0, 0, 0, time.UTC)
	createdDate := truncateToDate(created) // 2026-07-01 00:00
	due := time.Date(2026, 8, 5, 18, 0, 0, 0, time.UTC)
	dueDate := truncateToDate(due) // 2026-08-05 00:00

	t.Run("fixed window uses full creation→due span", func(t *testing.T) {
		g := db.GetGoalRow{
			Measurement: MeasurementHabit,
			CreatedAt:   ts(created),
			DueDate:     ts(due),
		}
		from, to := habitWindow(g, now)
		if !from.Equal(createdDate) {
			t.Fatalf("from = %v, want %v", from, createdDate)
		}
		if !to.Equal(dueDate) {
			t.Fatalf("to = %v, want %v", to, dueDate)
		}
	})

	t.Run("rolling 30-day window without due date", func(t *testing.T) {
		g := db.GetGoalRow{
			Measurement: MeasurementHabit,
			CreatedAt:   ts(created),
			DueDate:     pgtype.Timestamptz{Valid: false},
		}
		from, to := habitWindow(g, now)
		wantFrom := nowDate.AddDate(0, 0, -30)
		if !from.Equal(wantFrom) {
			t.Fatalf("from = %v, want %v", from, wantFrom)
		}
		if !to.Equal(nowDate) {
			t.Fatalf("to = %v, want %v", to, nowDate)
		}
	})

	t.Run("due date in future uses full span (not capped at now)", func(t *testing.T) {
		futureDue := now.AddDate(0, 0, 10)
		futureDueDate := truncateToDate(futureDue)
		g := db.GetGoalRow{
			Measurement: MeasurementHabit,
			CreatedAt:   ts(created),
			DueDate:     ts(futureDue),
		}
		_, to := habitWindow(g, now)
		// Window goes all the way to due_date, not capped at now.
		if !to.Equal(futureDueDate) {
			t.Fatalf("to = %v, want %v (full span to due date)", to, futureDueDate)
		}
	})

	t.Run("time components truncated to midnight", func(t *testing.T) {
		g := db.GetGoalRow{
			Measurement: MeasurementHabit,
			CreatedAt:   ts(time.Date(2026, 7, 1, 23, 59, 59, 0, time.UTC)),
			DueDate:     ts(time.Date(2026, 7, 10, 6, 0, 0, 0, time.UTC)),
		}
		from, to := habitWindow(g, now)
		if from.Hour() != 0 || from.Minute() != 0 {
			t.Fatalf("from not truncated: %v", from)
		}
		if to.Hour() != 0 || to.Minute() != 0 {
			t.Fatalf("to not truncated: %v", to)
		}
	})
}

func TestCalendarDays(t *testing.T) {
	cases := []struct {
		name string
		from time.Time
		to   time.Time
		want int64
	}{
		{"same day", truncateToDate(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)), truncateToDate(time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)), 1},
		{"two days", truncateToDate(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), truncateToDate(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)), 2},
		{"one week", truncateToDate(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), truncateToDate(time.Date(2026, 1, 7, 0, 0, 0, 0, time.UTC)), 7},
		{"90 days", truncateToDate(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), truncateToDate(time.Date(2026, 3, 31, 0, 0, 0, 0, time.UTC)), 90},
		{"reversed", truncateToDate(time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)), truncateToDate(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := calendarDays(tc.from, tc.to)
			if got != tc.want {
				t.Fatalf("calendarDays = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestTruncateToDate(t *testing.T) {
	utc := time.Date(2026, 8, 9, 14, 30, 45, 123, time.UTC)
	got := truncateToDate(utc)
	want := time.Date(2026, 8, 9, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("truncateToDate = %v, want %v", got, want)
	}
}

func TestTruncateToDateNonUTC(t *testing.T) {
	// A time in a non-UTC zone should still truncate to midnight UTC of the
	// same calendar date (not midnight in the source zone).
	loc, _ := time.LoadLocation("America/New_York")
	// 2026-08-09 22:00 EDT = 2026-08-10 02:00 UTC. Truncating to UTC date
	// yields 2026-08-10 00:00 UTC, NOT 2026-08-09 00:00.
	ny := time.Date(2026, 8, 9, 22, 0, 0, 0, loc)
	got := truncateToDate(ny)
	want := time.Date(2026, 8, 10, 0, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("truncateToDate(ny) = %v, want %v (UTC-normalized)", got, want)
	}
}

func TestCalendarDaysDSTSafe(t *testing.T) {
	// A 7-day span crossing a spring-forward DST boundary (2026-03-08 in
	// America/New_York). If calendarDays used wall-clock subtraction in a
	// DST zone, the 23-hour day would produce 6 instead of 7. UTC
	// normalization makes this correct.
	loc, _ := time.LoadLocation("America/New_York")
	from := time.Date(2026, 3, 7, 0, 0, 0, 0, loc) // before DST
	to := time.Date(2026, 3, 13, 0, 0, 0, 0, loc)  // after DST
	got := calendarDays(from, to)
	if got != 7 {
		t.Fatalf("calendarDays across DST = %d, want 7", got)
	}
}
