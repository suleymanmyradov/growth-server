package habitslogic

import (
	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func habitToProto(h db.Habit) *client.Habit {
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
		Streak:         h.Streak,
		Completed:      h.Completed,
		CompletedToday: h.Completed,
		CreatedAt:      h.CreatedAt.Time.Unix(),
		UpdatedAt:      h.UpdatedAt.Time.Unix(),
	}
}

func protoToHabitParams(name, description, category string, userID uuid.UUID) (string, *string, string, uuid.UUID) {
	var desc *string
	if description != "" {
		desc = &description
	}
	return name, desc, category, userID
}
