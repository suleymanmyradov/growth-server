package analytics

import (
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/analytics"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

// PatternDetection wraps the pkg/analytics service and handles DB-to-domain mapping.
type PatternDetection struct {
	inner *analytics.PatternDetectionService
}

// NewPatternDetection creates a new PatternDetection adapter.
func NewPatternDetection() *PatternDetection {
	return &PatternDetection{
		inner: &analytics.PatternDetectionService{},
	}
}

// AnalyzeLite maps db types to domain types and returns flat insights.
// streakByHabit supplies the derived (history-based) streak for each habit,
// since the streak is no longer stored on the habit row.
//
// Pattern detection is not cached here: the assembled personalization context
// that calls this is itself cached upstream (Redis read-through), so AnalyzeLite
// only runs on a context cache miss/refresh.
func (p *PatternDetection) AnalyzeLite(checkIns []db.CheckIn, habits []db.GetHabitRow, streakByHabit map[uuid.UUID]int32, userLoc *time.Location) map[string]string {
	return p.inner.AnalyzeLite(mapCheckIns(checkIns), mapHabits(habits, streakByHabit), userLoc)
}


// AnalyzeFullFromData maps db types to domain types and returns rich insights.
func (p *PatternDetection) AnalyzeFullFromData(checkIns []db.CheckIn, habits []db.GetHabitRow, streakByHabit map[uuid.UUID]int32, userLoc *time.Location) *analytics.PatternInsights {
	return p.inner.AnalyzeFullFromData(mapCheckIns(checkIns), mapHabits(habits, streakByHabit), userLoc)
}

func mapCheckIns(checkIns []db.CheckIn) []analytics.CheckInData {
	result := make([]analytics.CheckInData, len(checkIns))
	for i, ci := range checkIns {
		data := analytics.CheckInData{
			Status:    string(ci.Status),
			CreatedAt: ci.CreatedAt.Time,
			HabitID:   ci.HabitID.String(),
		}
		if ci.Mood != nil {
			mood := string(*ci.Mood)
			data.Mood = &mood
		}
		if ci.Blocker != nil {
			blocker := string(*ci.Blocker)
			data.Blocker = &blocker
		}
		result[i] = data
	}
	return result
}

func mapHabits(habits []db.GetHabitRow, streakByHabit map[uuid.UUID]int32) []analytics.HabitData {
	result := make([]analytics.HabitData, len(habits))
	for i, h := range habits {
		streak := streakByHabit[h.ID]
		result[i] = analytics.HabitData{
			ID:     h.ID.String(),
			Name:   h.Name,
			Streak: &streak,
		}
	}
	return result
}
