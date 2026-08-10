package goalslogic

import (
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

// goalToProto builds the proto Goal from a DB row. relatedHabitIds is the list
// of habit IDs linked to this goal (from goal_habits table); pass nil/empty
// for a goal with no links. milestones is the list of milestone steps for this
// goal (from goal_milestones table); pass nil/empty for non-milestone goals.
func goalToProto(g db.GetGoalRow, relatedHabitIds []string, milestones []db.GoalMilestone) *client.Goal {
	description := ""
	if g.Description != nil {
		description = *g.Description
	}
	dueDate := int64(0)
	if g.DueDate.Valid {
		dueDate = g.DueDate.Time.Unix()
	}
	if relatedHabitIds == nil {
		relatedHabitIds = []string{}
	}
	unit := ""
	if g.Unit != nil {
		unit = *g.Unit
	}
	var pbMilestones []*client.GoalMilestone
	if milestones != nil {
		pbMilestones = make([]*client.GoalMilestone, len(milestones))
		for i, m := range milestones {
			pbMilestones[i] = milestoneToProto(m)
		}
	}
	return &client.Goal{
		Id:              g.ID.String(),
		UserId:          g.UserID.String(),
		Title:           g.Title,
		Description:     description,
		Category:        g.Category,
		Progress:        g.Progress,
		Completed:       g.Completed,
		DueDate:         dueDate,
		CreatedAt:       g.CreatedAt.Time.Unix(),
		UpdatedAt:       g.UpdatedAt.Time.Unix(),
		RelatedHabitIds: relatedHabitIds,
		Measurement:     g.Measurement,
		StartValue:      numericToFloat(g.StartValue),
		CurrentValue:    numericToFloat(g.CurrentValue),
		TargetValue:     numericToFloat(g.TargetValue),
		Unit:            unit,
		Milestones:      pbMilestones,
	}
}

func milestoneToProto(m db.GoalMilestone) *client.GoalMilestone {
	doneAt := int64(0)
	if m.DoneAt.Valid {
		doneAt = m.DoneAt.Time.Unix()
	}
	return &client.GoalMilestone{
		Id:        m.ID.String(),
		GoalId:    m.GoalID.String(),
		Title:     m.Title,
		SortOrder: m.SortOrder,
		DoneAt:    doneAt,
	}
}

// protoToGoalParams converts the create request fields to db.CreateGoalParams.
func protoToGoalParams(title, description, category string, dueDate int64, userID uuid.UUID,
	measurement string, startValue, currentValue, targetValue float64, unit string) db.CreateGoalParams {
	var desc *string
	if description != "" {
		desc = &description
	}
	var dueTime pgtype.Timestamptz
	if dueDate > 0 {
		dueTime = pgtype.Timestamptz{Time: time.Unix(dueDate, 0), Valid: true}
	}
	var unitPtr *string
	if unit != "" {
		unitPtr = &unit
	}
	if measurement == "" {
		measurement = MeasurementManual
	}
	return db.CreateGoalParams{
		Title:        title,
		Description:  desc,
		Slug:         category,
		DueDate:      dueTime,
		UserID:       userID,
		Measurement:  measurement,
		StartValue:   floatToNumeric(startValue),
		CurrentValue: floatToNumeric(currentValue),
		TargetValue:  floatToNumeric(targetValue),
		Unit:         unitPtr,
	}
}

// floatToNumeric converts a float64 to a pgtype.Numeric via its string form.
func floatToNumeric(f float64) pgtype.Numeric {
	n := pgtype.Numeric{}
	_ = n.Scan(strconv.FormatFloat(f, 'f', -1, 64))
	return n
}

// parseHabitIDs converts a slice of string habit IDs to uuid.UUIDs, skipping
// any that are invalid (defensive — the client should send valid UUIDs).
func parseHabitIDs(ids []string) []uuid.UUID {
	out := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		if u, err := uuid.Parse(id); err == nil {
			out = append(out, u)
		}
	}
	return out
}

// habitUUIDsToStrings converts a slice of uuid.UUIDs to strings.
func habitUUIDsToStrings(ids []uuid.UUID) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		out = append(out, id.String())
	}
	return out
}
