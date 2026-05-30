package analytics

import (
	"time"

	"github.com/suleymanmyradov/growth-server/pkg/analytics"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
)

// PatternDetection wraps the pkg/analytics service and handles DB-to-domain mapping.
type PatternDetection struct {
	inner *analytics.PatternDetectionService
}

// NewPatternDetection creates a new PatternDetection adapter.
func NewPatternDetection() *PatternDetection {
	return &PatternDetection{inner: &analytics.PatternDetectionService{}}
}

// AnalyzeLite maps db types to domain types and returns flat insights.
func (p *PatternDetection) AnalyzeLite(checkIns []db.CheckIn, habits []db.Habit, userLoc *time.Location) map[string]string {
	return p.inner.AnalyzeLite(mapCheckIns(checkIns), mapHabits(habits), userLoc)
}

// AnalyzeFullFromData maps db types to domain types and returns rich insights.
func (p *PatternDetection) AnalyzeFullFromData(checkIns []db.CheckIn, habits []db.Habit, userLoc *time.Location) *analytics.PatternInsights {
	return p.inner.AnalyzeFullFromData(mapCheckIns(checkIns), mapHabits(habits), userLoc)
}

func mapCheckIns(checkIns []db.CheckIn) []analytics.CheckInData {
	result := make([]analytics.CheckInData, len(checkIns))
	for i, ci := range checkIns {
		data := analytics.CheckInData{
			Status:    string(ci.Status),
			CreatedAt: ci.CreatedAt,
			HabitID:   ci.HabitID.String(),
		}
		if ci.Mood.Valid {
			mood := string(ci.Mood.MoodType)
			data.Mood = &mood
		}
		if ci.Blocker.Valid {
			blocker := string(ci.Blocker.BlockerType)
			data.Blocker = &blocker
		}
		result[i] = data
	}
	return result
}

func mapHabits(habits []db.Habit) []analytics.HabitData {
	result := make([]analytics.HabitData, len(habits))
	for i, h := range habits {
		data := analytics.HabitData{
			ID:   h.ID.String(),
			Name: h.Name,
		}
		if h.Streak.Valid {
			streak := h.Streak.Int32
			data.Streak = &streak
		}
		result[i] = data
	}
	return result
}
