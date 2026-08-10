package goalslogic

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/zeromicro/go-zero/core/trace"
)

// Measurement type constants. Kept as strings to match the DB CHECK constraint
// and proto string field; validated at the logic layer.
const (
	MeasurementBinary    = "binary"
	MeasurementNumeric   = "numeric"
	MeasurementMilestone = "milestone"
	MeasurementHabit     = "habit"
	MeasurementManual    = "manual"
)

// validMeasurement reports whether m is a recognized measurement type.
func validMeasurement(m string) bool {
	switch m {
	case MeasurementBinary, MeasurementNumeric, MeasurementMilestone, MeasurementHabit, MeasurementManual:
		return true
	}
	return false
}

// ProgressInputs holds the derived inputs needed to compute progress for the
// non-manual measurement types. Fields are only read for the relevant type.
type ProgressInputs struct {
	// Milestone
	MilestoneTotal int32
	MilestoneDone  int32
	// Habit
	HabitCompletedDays int64 // distinct days with a completed check-in in window
	HabitExpectedDays  int64 // days in the goal window (rolling 30 if no due date)
}

// numericToFloat converts a pgtype.Numeric to a float64, returning 0 on
// invalid/empty values (defensive — the DB defaults start/current to 0).
func numericToFloat(n pgtype.Numeric) float64 {
	if !n.Valid {
		return 0
	}
	f, err := n.Float64Value()
	if err != nil {
		return 0
	}
	return f.Float64
}

// clampInt32 clamps v to the [lo, hi] range.
func clampInt32(v, lo, hi int32) int32 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// ComputeProgress returns a 0-100 progress value for a goal, derived from its
// measurement type. It is a pure function over already-fetched inputs so it
// can be unit-tested without a database.
//
//   - binary    → 0 or 100 from the goal's completed flag
//   - numeric   → clamp(0,100, round(100*(current-start)/(target-start)));
//     works for descending goals (weight loss) because both
//     numerator and denominator flip sign
//   - milestone → 100 * done / total; 0 when total == 0
//   - habit     → 100 * completedDays / expectedDays; 0 when no habits linked
//   - manual    → passthrough (the stored progress value)
func ComputeProgress(g db.GetGoalRow, m ProgressInputs) int32 {
	switch g.Measurement {
	case MeasurementBinary:
		if g.Completed {
			return 100
		}
		return 0

	case MeasurementNumeric:
		start := numericToFloat(g.StartValue)
		current := numericToFloat(g.CurrentValue)
		target := numericToFloat(g.TargetValue)
		if target == start {
			// Prevented by the DB CHECK constraint, but guard anyway.
			return 0
		}
		pct := 100.0 * (current - start) / (target - start)
		return clampInt32(int32(pct+0.5), 0, 100)

	case MeasurementMilestone:
		if m.MilestoneTotal <= 0 {
			return 0
		}
		pct := 100.0 * float64(m.MilestoneDone) / float64(m.MilestoneTotal)
		return clampInt32(int32(pct+0.5), 0, 100)

	case MeasurementHabit:
		if m.HabitExpectedDays <= 0 {
			return 0
		}
		pct := 100.0 * float64(m.HabitCompletedDays) / float64(m.HabitExpectedDays)
		return clampInt32(int32(pct+0.5), 0, 100)

	default: // manual
		return g.Progress
	}
}

// habitWindow returns the [from, to] date range for habit-driven progress.
//
// For goals with a due date: the window is [created_at, due_date] — the full
// planned duration. The denominator is the total calendar days in that span,
// so checking in once on day 1 of a 90-day goal yields ~1%, not 100%.
// Note: this means a habit goal with a due date can never reach 100% before
// the due date, even with perfect adherence — "45 of 90 planned days" = 50%.
// Auto-completion only happens when the due date arrives and every day was
// checked in. This is a deliberate product decision: habit goals measure
// consistency over the planned period, not completion of a one-time target.
//
// For goals without a due date: a rolling 31-day window (30 days back from
// today, inclusive of both endpoints) ending today, so the number reflects
// recent consistency rather than progress to infinity.
//
// Both endpoints are truncated to date (midnight UTC) to match
// CountCompletedCheckInDays, which counts distinct local_date values.
// NOTE: local_date is written in the user's timezone, while now() is server
// time (UTC). For users ahead of UTC, the boundary day may be off by one.
// This is a known limitation — fixing it requires the user's timezone, which
// is not available in the goal row.
func habitWindow(g db.GetGoalRow, now time.Time) (from, to time.Time) {
	if g.DueDate.Valid {
		// Fixed window: creation → due date (full planned duration).
		start := truncateToDate(g.CreatedAt.Time)
		end := truncateToDate(g.DueDate.Time)
		if start.After(end) {
			start = end
		}
		return start, end
	}
	// Rolling window: 30 days back from today.
	end := truncateToDate(now)
	return end.AddDate(0, 0, -30), end
}

// truncateToDate strips the time component, returning midnight of the same
// calendar day in UTC. Normalizing to UTC avoids DST-fragility: a span
// crossing a spring-forward transition in a non-UTC location would have a
// 23-hour "day" that breaks the /24 arithmetic in calendarDays.
func truncateToDate(t time.Time) time.Time {
	t = t.UTC()
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
}

// calendarDays returns the number of days between from and to, inclusive of
// both endpoints. Inputs are truncated to midnight UTC internally, so callers
// can pass raw timestamps. UTC normalization avoids DST-fragility: a span
// crossing a spring-forward transition in a non-UTC location would have a
// 23-hour "day" that breaks naive /24 arithmetic.
func calendarDays(from, to time.Time) int64 {
	from = truncateToDate(from)
	to = truncateToDate(to)
	if to.Before(from) {
		return 0
	}
	// 24h per day in UTC (no DST), +1 to include both endpoints.
	return int64(to.Sub(from).Hours()/24) + 1
}

// recomputeAndPersist loads the inputs for a goal's measurement type, computes
// progress, and writes it to goals.progress. Idempotent by construction:
// it recomputes from source rows, so a retried call converges to the same value.
func recomputeAndPersist(ctx context.Context, svcCtx *svc.ServiceContext, goalID uuid.UUID) (db.GetGoalRow, error) {
	return RecomputeGoalProgressWithRepo(ctx, svcCtx.Repo.Goals, svcCtx.Repo.CheckIns, goalID)
}

// RecomputeGoalProgressWithRepo is the transaction-aware variant: pass the
// Goals and CheckIns repos backed by the current transaction (from
// svc.WithTx(tx).Goals / .CheckIns) so the recompute sees uncommitted writes
// from the triggering operation. Exported so other domains (e.g.
// checkinservice) can recompute affected goals inside their own transactions
// without importing the goals logic internals.
func RecomputeGoalProgressWithRepo(ctx context.Context, goals repository.IGoals, checkIns repository.ICheckIns, goalID uuid.UUID) (db.GetGoalRow, error) {
	ctx, span := trace.TracerFromContext(ctx).Start(ctx, "goalslogic.RecomputeGoalProgressWithRepo")
	defer span.End()

	goal, err := goals.GetGoalByID(ctx, goalID)
	if err != nil {
		return goal, err
	}

	inputs := ProgressInputs{}
	switch goal.Measurement {
	case MeasurementMilestone:
		counts, err := goals.CountGoalMilestones(ctx, goalID)
		if err != nil {
			return goal, err
		}
		inputs.MilestoneTotal = int32(counts.Total)
		inputs.MilestoneDone = int32(counts.Done)

	case MeasurementHabit:
		habitIDs, err := goals.ListGoalHabitIDsByGoal(ctx, goalID)
		if err != nil {
			return goal, err
		}
		if len(habitIDs) > 0 {
			from, to := habitWindow(goal, time.Now())
			days, err := checkIns.CountCompletedCheckInDays(ctx, habitIDs,
				pgtype.Date{Time: from, Valid: true},
				pgtype.Date{Time: to, Valid: true})
			if err != nil {
				return goal, err
			}
			inputs.HabitCompletedDays = days
			inputs.HabitExpectedDays = calendarDays(from, to)
			if inputs.HabitExpectedDays < 1 {
				inputs.HabitExpectedDays = 1
			}
		}
	}

	progress := ComputeProgress(goal, inputs)
	return goals.RecomputeGoalProgress(ctx, goalID, progress)
}
