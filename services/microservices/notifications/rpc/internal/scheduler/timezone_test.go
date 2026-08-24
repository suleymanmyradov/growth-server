package scheduler

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestNextOccurrence_TodayStillAhead(t *testing.T) {
	now := time.Date(2025, 5, 16, 8, 0, 0, 0, time.UTC) // 08:00 UTC
	loc, _ := time.LoadLocation("America/New_York")     // UTC-4/5

	// check_in_time = 10:00 local = 14:00/15:00 UTC → still ahead
	cit := pgtype.Time{Microseconds: 10 * 3600 * 1_000_000, Valid: true}

	got, err := NextOccurrence(now, "America/New_York", cit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	localGot := got.In(loc)
	if localGot.Hour() != 10 || localGot.Minute() != 0 {
		t.Errorf("expected 10:00 local, got %s", localGot.Format(time.RFC3339))
	}
	if localGot.Before(now.In(loc)) {
		t.Errorf("expected future time, got %s", localGot)
	}
}

func TestNextOccurrence_Tomorrow(t *testing.T) {
	now := time.Date(2025, 5, 16, 22, 0, 0, 0, time.UTC) // 22:00 UTC
	loc, _ := time.LoadLocation("America/New_York")      // 18:00 local

	// check_in_time = 10:00 local → already passed today
	cit := pgtype.Time{Microseconds: 10 * 3600 * 1_000_000, Valid: true}

	got, err := NextOccurrence(now, "America/New_York", cit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	localGot := got.In(loc)
	if localGot.Hour() != 10 {
		t.Errorf("expected hour 10, got %d", localGot.Hour())
	}
	if localGot.Day() != 17 { // next day
		t.Errorf("expected next day, got %s", localGot.Format(time.RFC3339))
	}
}

func TestNextOccurrence_NullCheckInTime(t *testing.T) {
	now := time.Date(2025, 5, 16, 8, 0, 0, 0, time.UTC)
	_, err := NextOccurrence(now, "UTC", pgtype.Time{Valid: false})
	if err == nil {
		t.Fatal("expected error for null check_in_time")
	}
}

func TestNextOccurrence_InvalidTimezone(t *testing.T) {
	now := time.Date(2025, 5, 16, 8, 0, 0, 0, time.UTC)
	cit := pgtype.Time{Microseconds: 10 * 3600 * 1_000_000, Valid: true}

	// Invalid timezone should return an error.
	_, err := NextOccurrence(now, "Invalid/Zone", cit)
	if err == nil {
		t.Fatal("expected error for invalid timezone")
	}
}

func TestNextDailyAt(t *testing.T) {
	now := time.Date(2025, 5, 16, 23, 30, 0, 0, time.UTC)
	got, err := NextDailyAt(now, "America/New_York", 20, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	loc, _ := time.LoadLocation("America/New_York")
	local := got.In(loc)
	if local.Hour() != 20 || local.Minute() != 0 || !local.After(now.In(loc)) {
		t.Fatalf("expected next local 20:00, got %s", local)
	}
}

func TestNextDailyAt_InvalidTimezone(t *testing.T) {
	if _, err := NextDailyAt(time.Now(), "Invalid/Zone", 20, 0); err == nil {
		t.Fatal("expected invalid timezone error")
	}
}

func TestNextWeekday_SameDayAhead(t *testing.T) {
	// Wednesday 10:00 UTC → Wednesday 18:00 UTC is still ahead
	now := time.Date(2025, 5, 14, 10, 0, 0, 0, time.UTC) // Wed

	got, err := NextWeekday(now, "UTC", time.Wednesday, 18, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Weekday() != time.Wednesday {
		t.Errorf("expected Wednesday, got %s", got.Weekday())
	}
	if got.Hour() != 18 {
		t.Errorf("expected 18:00, got %d", got.Hour())
	}
}

func TestNextWeekday_SameDayPassed(t *testing.T) {
	// Wednesday 20:00 UTC → Wednesday 18:00 already passed → next week
	now := time.Date(2025, 5, 14, 20, 0, 0, 0, time.UTC) // Wed

	got, err := NextWeekday(now, "UTC", time.Wednesday, 18, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Weekday() != time.Wednesday {
		t.Errorf("expected Wednesday, got %s", got.Weekday())
	}
	daysDiff := got.Sub(now).Hours() / 24
	if daysDiff < 6 {
		t.Errorf("expected ~7 days ahead, got %.1f", daysDiff)
	}
}

func TestNextWeekday_DifferentDay(t *testing.T) {
	// Wednesday → Sunday
	now := time.Date(2025, 5, 14, 10, 0, 0, 0, time.UTC) // Wed

	got, err := NextWeekday(now, "UTC", time.Sunday, 18, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Weekday() != time.Sunday {
		t.Errorf("expected Sunday, got %s", got.Weekday())
	}
}

func TestNextCoachDigest_TodayStillAhead(t *testing.T) {
	// 08:00 UTC, check_in_time 09:00 UTC → digest at 11:00 UTC today
	now := time.Date(2025, 5, 16, 8, 0, 0, 0, time.UTC)
	cit := pgtype.Time{Microseconds: 9 * 3600 * 1_000_000, Valid: true}

	got, err := NextCoachDigest(now, "UTC", cit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Hour() != 11 {
		t.Errorf("expected 11:00 UTC (check_in_time + 2h), got %s", got.Format(time.RFC3339))
	}
	if got.Day() != 16 {
		t.Errorf("expected today, got %s", got.Format(time.RFC3339))
	}
}

func TestNextCoachDigest_Tomorrow(t *testing.T) {
	// 15:00 UTC, check_in_time 09:00 UTC → digest at 11:00 already passed → tomorrow
	now := time.Date(2025, 5, 16, 15, 0, 0, 0, time.UTC)
	cit := pgtype.Time{Microseconds: 9 * 3600 * 1_000_000, Valid: true}

	got, err := NextCoachDigest(now, "UTC", cit)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Hour() != 11 {
		t.Errorf("expected 11:00 UTC, got %s", got.Format(time.RFC3339))
	}
	if got.Day() != 17 {
		t.Errorf("expected tomorrow, got %s", got.Format(time.RFC3339))
	}
}

func TestNextCoachDigest_NullCheckInTime(t *testing.T) {
	now := time.Date(2025, 5, 16, 8, 0, 0, 0, time.UTC)
	_, err := NextCoachDigest(now, "UTC", pgtype.Time{Valid: false})
	if err == nil {
		t.Fatal("expected error for null check_in_time")
	}
}

func TestGoalDeadlineReminderTime_MorningAhead(t *testing.T) {
	// Deadline at 18:00 UTC on May 20. Now is May 20 06:00 UTC.
	// 09:00 UTC is still ahead → reminder at 09:00 UTC.
	now := time.Date(2025, 5, 20, 6, 0, 0, 0, time.UTC)
	deadline := time.Date(2025, 5, 20, 18, 0, 0, 0, time.UTC)

	got, err := GoalDeadlineReminderTime(now, deadline, "UTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Hour() != 9 || got.Day() != 20 {
		t.Errorf("expected 09:00 UTC May 20, got %s", got.Format(time.RFC3339))
	}
}

func TestGoalDeadlineReminderTime_MorningPassed_OneHourBefore(t *testing.T) {
	// Deadline at 18:00 UTC. Now is 15:00 UTC. 09:00 passed, but 17:00 (1h
	// before) is still ahead → reminder at 17:00 UTC.
	now := time.Date(2025, 5, 20, 15, 0, 0, 0, time.UTC)
	deadline := time.Date(2025, 5, 20, 18, 0, 0, 0, time.UTC)

	got, err := GoalDeadlineReminderTime(now, deadline, "UTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Hour() != 17 {
		t.Errorf("expected 17:00 UTC (deadline-1h), got %s", got.Format(time.RFC3339))
	}
}

func TestGoalDeadlineReminderTime_AllPassed_ReturnsDeadline(t *testing.T) {
	// Deadline at 18:00 UTC. Now is 17:30 UTC. Both 09:00 and 17:00 passed.
	// Return the deadline itself (scheduler fires immediately).
	now := time.Date(2025, 5, 20, 17, 30, 0, 0, time.UTC)
	deadline := time.Date(2025, 5, 20, 18, 0, 0, 0, time.UTC)

	got, err := GoalDeadlineReminderTime(now, deadline, "UTC")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got.Equal(deadline.UTC()) {
		t.Errorf("expected deadline %s, got %s", deadline.UTC(), got)
	}
}

func TestGoalDeadlineReminderTime_TimezoneAware(t *testing.T) {
	// Deadline at 18:00 UTC = 14:00 EDT (America/New_York, UTC-4 in May).
	// Now is 06:00 UTC = 02:00 EDT. 09:00 EDT = 13:00 UTC is ahead.
	now := time.Date(2025, 5, 20, 6, 0, 0, 0, time.UTC)
	deadline := time.Date(2025, 5, 20, 18, 0, 0, 0, time.UTC)

	got, err := GoalDeadlineReminderTime(now, deadline, "America/New_York")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	loc, _ := time.LoadLocation("America/New_York")
	local := got.In(loc)
	if local.Hour() != 9 {
		t.Errorf("expected 09:00 EDT, got %s", local.Format(time.RFC3339))
	}
}

func TestGoalDeadlineReminderTime_ZeroDeadline(t *testing.T) {
	_, err := GoalDeadlineReminderTime(time.Now(), time.Time{}, "UTC")
	if err == nil {
		t.Fatal("expected error for zero deadline")
	}
}

func TestGoalDeadlineReminderTime_InvalidTimezoneFallbackUTC(t *testing.T) {
	now := time.Date(2025, 5, 20, 6, 0, 0, 0, time.UTC)
	deadline := time.Date(2025, 5, 20, 18, 0, 0, 0, time.UTC)

	got, err := GoalDeadlineReminderTime(now, deadline, "Invalid/Zone")
	if err != nil {
		t.Fatalf("expected fallback to UTC, got error: %v", err)
	}
	if got.Hour() != 9 {
		t.Errorf("expected 09:00 UTC fallback, got %s", got.Format(time.RFC3339))
	}
}
