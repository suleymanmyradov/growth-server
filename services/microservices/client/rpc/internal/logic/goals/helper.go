package goalslogic

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

// goalToProto builds the proto Goal from a DB row. relatedHabitIds is the list
// of habit IDs linked to this goal (from goal_habits table); pass nil/empty
// for a goal with no links.
func goalToProto(g db.GetGoalRow, relatedHabitIds []string) *client.Goal {
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
	}
}

func protoToGoalParams(title, description, category string, dueDate int64, userID uuid.UUID) db.CreateGoalParams {
	var desc *string
	if description != "" {
		desc = &description
	}
	var dueTime pgtype.Timestamptz
	if dueDate > 0 {
		dueTime = pgtype.Timestamptz{Time: time.Unix(dueDate, 0), Valid: true}
	}
	return db.CreateGoalParams{
		Title:       title,
		Description: desc,
		Slug:        category,
		DueDate:     dueTime,
		UserID:      userID,
	}
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
