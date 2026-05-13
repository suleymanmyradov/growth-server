package goalslogic

import (
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func goalToProto(g db.Goal) *client.Goal {
	return &client.Goal{
		Id:          g.ID.String(),
		UserId:      g.UserID.String(),
		Title:       g.Title,
		Description: g.Description.String,
		Category:    g.Category,
		Progress:    g.Progress.Int32,
		Completed:   g.Completed.Bool,
		DueDate:     g.DueDate.Time.Unix(),
		CreatedAt:   g.CreatedAt.Unix(),
		UpdatedAt:   g.UpdatedAt.Unix(),
	}
}

func protoToGoalParams(title, description, category string, dueDate int64, userID uuid.UUID) db.CreateGoalParams {
	var dueTime time.Time
	if dueDate > 0 {
		dueTime = time.Unix(dueDate, 0)
	}
	return db.CreateGoalParams{
		Title:       title,
		Description: sql.NullString{String: description, Valid: description != ""},
		Category:    category,
		DueDate:     sql.NullTime{Time: dueTime, Valid: dueDate > 0},
		UserID:      userID,
	}
}
