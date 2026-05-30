package checkinservicelogic

import (
	"database/sql"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func checkInToProto(c db.CheckIn) *client.CheckIn {
	mood := ""
	if c.Mood.Valid {
		mood = string(c.Mood.MoodType)
	}
	energy := ""
	if c.Energy.Valid {
		energy = string(c.Energy.EnergyLevel)
	}
	blocker := ""
	if c.Blocker.Valid {
		blocker = string(c.Blocker.BlockerType)
	}
	return &client.CheckIn{
		Id:        c.ID.String(),
		UserId:    c.UserID.String(),
		HabitId:   c.HabitID.String(),
		Status:    string(c.Status),
		Mood:      mood,
		Energy:    energy,
		Blocker:   blocker,
		Note:      c.Note.String,
		CreatedAt: c.CreatedAt.Unix(),
	}
}

func protoToCheckInParams(userID, habitID uuid.UUID, status, mood, energy, blocker, note string) db.CreateCheckInParams {
	return db.CreateCheckInParams{
		UserID:  userID,
		HabitID: habitID,
		Status:  db.CheckInStatus(status),
		Mood:    db.NullMoodType{MoodType: db.MoodType(mood), Valid: mood != ""},
		Energy:  db.NullEnergyLevel{EnergyLevel: db.EnergyLevel(energy), Valid: energy != ""},
		Blocker: db.NullBlockerType{BlockerType: db.BlockerType(blocker), Valid: blocker != ""},
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
