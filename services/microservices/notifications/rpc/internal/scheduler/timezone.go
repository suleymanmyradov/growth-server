package scheduler

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

// NextOccurrence computes the next occurrence of the user's check_in_time in
// their IANA timezone. If the time today has already passed, tomorrow's
// occurrence is returned. Falls back to UTC with a logged warning on invalid
// timezone.
func NextOccurrence(now time.Time, tzName string, checkInTime pgtype.Time) (time.Time, error) {
	loc, err := safeLocation(tzName)
	if err != nil {
		return time.Time{}, err
	}

	if !checkInTime.Valid {
		return time.Time{}, fmt.Errorf("check_in_time is null")
	}

	localNow := now.In(loc)
	ref := time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)
	checkTime := ref.Add(time.Duration(checkInTime.Microseconds) * time.Microsecond)
	h, m, s := checkTime.Clock()
	targetToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), h, m, s, 0, loc)

	if targetToday.After(localNow) {
		return targetToday.UTC(), nil
	}
	return targetToday.AddDate(0, 0, 1).UTC(), nil
}

// NextWeekday computes the next occurrence of the given weekday at hour:min in
// the user's IANA timezone. If today is the target weekday and the time has
// not yet passed, today is returned; otherwise the following week's occurrence.
func NextDailyAt(now time.Time, tzName string, hour, min int) (time.Time, error) {
	loc, err := safeLocation(tzName)
	if err != nil {
		return time.Time{}, err
	}

	localNow := now.In(loc)
	candidate := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), hour, min, 0, 0, loc)
	if !candidate.After(localNow) {
		candidate = candidate.AddDate(0, 0, 1)
	}
	return candidate.UTC(), nil
}

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

// NextCoachDigest computes the next occurrence of the daily coach digest,
// which fires 2 hours after the user's check_in_time in their timezone. This
// gives the user a window to check in on all their habits before the coach
// reviews the day. If today's digest time has already passed, tomorrow's is
// returned.
func NextCoachDigest(now time.Time, tzName string, checkInTime pgtype.Time) (time.Time, error) {
	loc, err := safeLocation(tzName)
	if err != nil {
		return time.Time{}, err
	}

	if !checkInTime.Valid {
		return time.Time{}, fmt.Errorf("check_in_time is null")
	}

	localNow := now.In(loc)
	ref := time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC)
	checkTime := ref.Add(time.Duration(checkInTime.Microseconds) * time.Microsecond)
	h, m, s := checkTime.Clock()
	targetToday := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), h, m, s, 0, loc).
		Add(2 * time.Hour)

	if targetToday.After(localNow) {
		return targetToday.UTC(), nil
	}
	return targetToday.AddDate(0, 0, 1).UTC(), nil
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

// GoalDeadlineReminderTime computes when to send the goal-deadline reminder for
// a goal with the given deadline. The reminder fires at 09:00 on the deadline
// day in the user's timezone, giving them the morning to act. If 09:00 has
// already passed on the deadline day, the reminder is scheduled 1 hour before
// the deadline instead. If even that is in the past, the deadline itself is
// returned (the scheduler will fire it immediately). Returns an error for an
// empty/zero deadline; an invalid timezone falls back to UTC.
func GoalDeadlineReminderTime(now time.Time, deadline time.Time, tzName string) (time.Time, error) {
	if deadline.IsZero() {
		return time.Time{}, fmt.Errorf("deadline is zero")
	}
	loc, err := safeLocation(tzName)
	if err != nil {
		// Fall back to UTC rather than failing — a bad timezone should not
		// prevent the reminder from being scheduled.
		loc = time.UTC
	}
	localDeadline := deadline.In(loc)
	morning := time.Date(localDeadline.Year(), localDeadline.Month(), localDeadline.Day(), 9, 0, 0, 0, loc)
	if morning.After(now) {
		return morning.UTC(), nil
	}
	oneHourBefore := localDeadline.Add(-time.Hour)
	if oneHourBefore.After(now) {
		return oneHourBefore.UTC(), nil
	}
	return deadline.UTC(), nil
}
