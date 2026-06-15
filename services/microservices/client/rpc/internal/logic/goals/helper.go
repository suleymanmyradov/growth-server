package goalslogic

import (
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func goalToProto(g db.GetGoalRow) *client.Goal {
	description := ""
	if g.Description != nil {
		description = *g.Description
	}
	dueDate := int64(0)
	if g.DueDate.Valid {
		dueDate = g.DueDate.Time.Unix()
	}
	return &client.Goal{
		Id:          g.ID.String(),
		UserId:      g.UserID.String(),
		Title:       g.Title,
		Description: description,
		Category:    g.Category,
		Progress:    g.Progress,
		Completed:   g.Completed,
		DueDate:     dueDate,
		CreatedAt:   g.CreatedAt.Time.Unix(),
		UpdatedAt:   g.UpdatedAt.Time.Unix(),
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
