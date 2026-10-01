package habitslogic

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/svc"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

// pgxTxRunner is satisfied by *postgres.PgxTxRunner; tests inject a no-op
// runner that calls fn(nil) so repository calls land on the mock svcCtx.Repo.
type pgxTxRunner interface {
	Run(ctx context.Context, userID string, fn func(pgx.Tx) error) error
}

// runInTx runs fn inside svcCtx.RunInTx in production; when override is set
// (tests only), fn runs directly against svcCtx.Repo without a real
// transaction — mirroring the billingservice testTxRunner seam.
func runInTx(svcCtx *svc.ServiceContext, override pgxTxRunner, ctx context.Context, userID string, fn func(*repository.Repository) error) error {
	if override != nil {
		return override.Run(ctx, userID, func(pgx.Tx) error {
			return fn(svcCtx.Repo)
		})
	}
	return svcCtx.RunInTx(ctx, userID, fn)
}

// habitToProto builds the proto Habit from a DB row. The streak is derived
// from check_ins history (not stored on the habit), so the caller must pass it
// in — use 0 for a freshly created habit, or the result of GetHabitStreak/
// GetHabitStreaks for existing habits.
func habitToProto(h db.GetHabitRow, streak int32, recentHistory []bool) *client.Habit {
	description := ""
	if h.Description != nil {
		description = *h.Description
	}
	return &client.Habit{
		Id:             h.ID.String(),
		UserId:         h.UserID.String(),
		Name:           h.Name,
		Description:    description,
		Category:       h.Category,
		Streak:         streak,
		Completed:      h.Completed,
		CompletedToday: h.Completed,
		CreatedAt:      h.CreatedAt.Time.Unix(),
		UpdatedAt:      h.UpdatedAt.Time.Unix(),
		RecentHistory:  recentHistory,
	}
}

func protoToHabitParams(name, description, category string, userID uuid.UUID) (string, *string, string, uuid.UUID) {
	var desc *string
	if description != "" {
		desc = &description
	}
	return name, desc, category, userID
}

// bucketHabitHistory groups completed-check-in history rows by habit id in a
// single pass (O(n)) so each habit's recent history can be built without
// rescanning the full row set.
func bucketHabitHistory(rows []db.ListHabitHistoryRow) map[uuid.UUID][]db.ListHabitHistoryRow {
	out := make(map[uuid.UUID][]db.ListHabitHistoryRow, len(rows))
	for _, r := range rows {
		if !r.LocalDate.Valid {
			continue
		}
		out[r.HabitID] = append(out[r.HabitID], r)
	}
	return out
}

// buildRecentHistory constructs a 28-element boolean slice (oldest first,
// index 0 = 27 days ago, index 27 = today) marking which days the habit was
// completed. `today` is the user's current calendar day; `rows` are the
// completed check-ins for this habit only within the window.
func buildRecentHistory(habitID uuid.UUID, today time.Time, rows []db.ListHabitHistoryRow) []bool {
	const days = 28
	out := make([]bool, days)
	start := utcMidnight(today).AddDate(0, 0, -(days - 1))
	for _, r := range rows {
		if r.HabitID != habitID || !r.LocalDate.Valid {
			continue
		}
		d := utcMidnight(r.LocalDate.Time)
		dayDiff := int(d.Sub(start).Hours() / 24)
		if dayDiff >= 0 && dayDiff < days {
			out[dayDiff] = true
		}
	}
	return out
}

// utcMidnight normalizes t to 00:00 UTC on the same calendar date. Both sides
// of the day-diff must share one location for the subtraction to be an exact
// multiple of 24h: pgtype.Date decodes at UTC midnight while `today` arrives
// in the user's location — diffing those directly loses the UTC offset and
// shifts every bucket a day for users west of UTC.
func utcMidnight(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// userToday returns "today" in the user's timezone, falling back to UTC.
func userToday(tz string) time.Time {
	loc, err := time.LoadLocation(tz)
	if err != nil || tz == "" {
		loc = time.UTC
	}
	return time.Now().In(loc)
}
