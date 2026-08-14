package goals

import (
	"strings"
	"time"

	"github.com/suleymanmyradov/growth-server/services/gateway/growth/internal/types"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

// rpcGoalToType converts an RPC Goal to the HTTP types.Goal, mapping all
// typed-measurement fields and milestones. Shared by all goal logic files to
// avoid duplication.
func rpcGoalToType(g *client.Goal) types.Goal {
	milestones := make([]types.GoalMilestone, 0, len(g.Milestones))
	for _, m := range g.Milestones {
		milestones = append(milestones, types.GoalMilestone{
			Id:        m.Id,
			GoalId:    m.GoalId,
			Title:     m.Title,
			SortOrder: int(m.SortOrder),
			DoneAt:    formatTime(m.DoneAt),
		})
	}
	return types.Goal{
		Id:              g.Id,
		Title:           g.Title,
		Description:     g.Description,
		Category:        g.Category,
		DueDate:         formatTime(g.DueDate),
		Progress:        int(g.Progress),
		Completed:       g.Completed,
		RelatedHabitIds: nonNilHabitIds(g.RelatedHabitIds),
		Measurement:     g.Measurement,
		StartValue:      g.StartValue,
		CurrentValue:    g.CurrentValue,
		TargetValue:     g.TargetValue,
		Unit:            g.Unit,
		Milestones:      milestones,
		UserId:          g.UserId,
		CreatedAt:       formatTime(g.CreatedAt),
		UpdatedAt:       formatTime(g.UpdatedAt),
	}
}

func formatTime(unix int64) string {
	if unix == 0 {
		return ""
	}
	return time.Unix(unix, 0).Format(time.RFC3339)
}

// parseDueDate converts a date string to a Unix timestamp (seconds). Accepts
// both RFC3339 ("2024-12-31T00:00:00Z") and date-only ("2024-12-31") formats.
// Returns 0 for empty strings (meaning "no due date" / "don't update").
func parseDueDate(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	// Try RFC3339 first (full timestamp from the API response).
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix()
	}
	// Fall back to date-only (from <input type="date">).
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Unix()
	}
	return 0
}

// nonNilHabitIds returns the slice if non-nil, otherwise an empty slice so
// JSON serialization produces [] instead of null.
func nonNilHabitIds(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}
