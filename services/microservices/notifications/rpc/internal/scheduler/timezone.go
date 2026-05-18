package scheduler

import (
	"database/sql"
	"fmt"
	"time"
)

// NextOccurrence computes the next occurrence of the user's check_in_time in
// their IANA timezone. If the time today has already passed, tomorrow's
// occurrence is returned. Falls back to UTC with a logged warning on invalid
// timezone.
func NextOccurrence(now time.Time, tzName string, checkInTime sql.NullTime) (time.Time, error) {
	loc, err := safeLocation(tzName)
	if err != nil {
		return time.Time{}, err
	}

	if !checkInTime.Valid {
		return time.Time{}, fmt.Errorf("check_in_time is null")
	}

	localNow := now.In(loc)
	h, m, s := checkInTime.Time.Clock()
	targetToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), h, m, s, 0, loc)

	if targetToday.After(localNow) {
		return targetToday.UTC(), nil
	}
	return targetToday.AddDate(0, 0, 1).UTC(), nil
}

// NextWeekday computes the next occurrence of the given weekday at hour:min in
// the user's IANA timezone. If today is the target weekday and the time has
// not yet passed, today is returned; otherwise the following week's occurrence.
func NextWeekday(now time.Time, tzName string, weekday time.Weekday, hour, min int) (time.Time, error) {
	loc, err := safeLocation(tzName)
	if err != nil {
		return time.Time{}, err
	}

	localNow := now.In(loc)
	daysAhead := int(weekday - localNow.Weekday())
	if daysAhead < 0 {
		daysAhead += 7
	}

	candidate := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), hour, min, 0, 0, loc).AddDate(0, 0, daysAhead)
	if candidate.Before(localNow) || candidate.Equal(localNow) {
		candidate = candidate.AddDate(0, 0, 7)
	}
	return candidate.UTC(), nil
}

// safeLocation parses the IANA timezone string. Returns an error for invalid
// timezone names so callers can decide how to handle the failure.
func safeLocation(tzName string) (*time.Location, error) {
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return nil, fmt.Errorf("invalid timezone %q: %w", tzName, err)
	}
	return loc, nil
}
