package checkinservicelogic

import (
	"database/sql"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func checkInToProto(c db.CheckIn) *client.CheckIn {
	return &client.CheckIn{
		Id:        c.ID.String(),
		UserId:    c.UserID.String(),
		HabitId:   c.HabitID.String(),
		Status:    c.Status,
		Mood:      c.Mood.String,
		Energy:    c.Energy.String,
		Blocker:   c.Blocker.String,
		Note:      c.Note.String,
		CreatedAt: c.CreatedAt.Unix(),
	}
}

func protoToCheckInParams(userID, habitID uuid.UUID, status, mood, energy, blocker, note string) db.CreateCheckInParams {
	return db.CreateCheckInParams{
		UserID:  userID,
		HabitID: habitID,
		Status:  status,
		Mood:    sql.NullString{String: mood, Valid: mood != ""},
		Energy:  sql.NullString{String: energy, Valid: energy != ""},
		Blocker: sql.NullString{String: blocker, Valid: blocker != ""},
		Note:    sql.NullString{String: note, Valid: note != ""},
	}
}

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
