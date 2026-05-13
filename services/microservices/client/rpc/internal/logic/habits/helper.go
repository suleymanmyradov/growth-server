package habitslogic

import (
	"database/sql"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func habitToProto(h db.Habit) *client.Habit {
	return &client.Habit{
		Id:             h.ID.String(),
		UserId:         h.UserID.String(),
		Name:           h.Name,
		Description:    h.Description.String,
		Category:       h.Category,
		Streak:         h.Streak.Int32,
		Completed:      h.Completed.Bool,
		CompletedToday: h.Completed.Bool,
		CreatedAt:      h.CreatedAt.Unix(),
		UpdatedAt:      h.UpdatedAt.Unix(),
	}
}

func protoToHabitParams(name, description, category string, userID uuid.UUID) db.CreateHabitParams {
	return db.CreateHabitParams{
		Name:        name,
		Description: sql.NullString{String: description, Valid: description != ""},
		Category:    category,
		UserID:      userID,
	}
}
