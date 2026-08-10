package goals

import (
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

// nonNilHabitIds returns the slice if non-nil, otherwise an empty slice so
// JSON serialization produces [] instead of null.
func nonNilHabitIds(ids []string) []string {
	if ids == nil {
		return []string{}
	}
	return ids
}
