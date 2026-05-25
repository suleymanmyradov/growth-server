package scheduler

import (
	"database/sql"
	"testing"
	"time"
)

func TestNextOccurrence_TodayStillAhead(t *testing.T) {
	now := time.Date(2025, 5, 16, 8, 0, 0, 0, time.UTC) // 08:00 UTC
	loc, _ := time.LoadLocation("America/New_York")     // UTC-4/5

	// check_in_time = 10:00 local = 14:00/15:00 UTC → still ahead
	cit := sql.NullTime{Time: time.Date(0, 0, 0, 10, 0, 0, 0, time.UTC), Valid: true}

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
	cit := sql.NullTime{Time: time.Date(0, 0, 0, 10, 0, 0, 0, time.UTC), Valid: true}

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
	_, err := NextOccurrence(now, "UTC", sql.NullTime{Valid: false})
	if err == nil {
		t.Fatal("expected error for null check_in_time")
	}
}

func TestNextOccurrence_InvalidTimezone(t *testing.T) {
	now := time.Date(2025, 5, 16, 8, 0, 0, 0, time.UTC)
	cit := sql.NullTime{Time: time.Date(0, 0, 0, 10, 0, 0, 0, time.UTC), Valid: true}

	// Invalid timezone should return an error.
	_, err := NextOccurrence(now, "Invalid/Zone", cit)
	if err == nil {
		t.Fatal("expected error for invalid timezone")
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
