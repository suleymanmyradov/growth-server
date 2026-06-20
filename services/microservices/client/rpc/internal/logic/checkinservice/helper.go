package checkinservicelogic

import (
	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/client/rpc/pb/client"
)

func checkInToProto(c db.CheckIn) *client.CheckIn {
	mood := ""
	if c.Mood != nil {
		mood = string(*c.Mood)
	}
	energy := ""
	if c.Energy != nil {
		energy = string(*c.Energy)
	}
	blocker := ""
	if c.Blocker != nil {
		blocker = string(*c.Blocker)
	}
	note := ""
	if c.Note != nil {
		note = *c.Note
	}
	return &client.CheckIn{
		Id:        c.ID.String(),
		UserId:    c.UserID.String(),
		HabitId:   c.HabitID.String(),
		Status:    string(c.Status),
		Mood:      mood,
		Energy:    energy,
		Blocker:   blocker,
		Note:      note,
		CreatedAt: c.CreatedAt.Time.Unix(),
	}
}

func protoToCheckInParams(userID, habitID uuid.UUID, status, mood, energy, blocker, note string) db.CreateCheckInParams {
	params := db.CreateCheckInParams{
		UserID:  userID,
		HabitID: habitID,
		Status:  (status),
	}
	if mood != "" {
		m := (mood)
		params.Mood = &m
	}
	if energy != "" {
		e := (energy)
		params.Energy = &e
	}
	if blocker != "" {
		b := (blocker)
		params.Blocker = &b
	}
	if note != "" {
		params.Note = &note
	}
	return params
}

// habitToProto builds the proto Habit from a DB row. The streak is derived
// from check_ins history (not stored on the habit), so the caller must pass
// it in.
func habitToProto(h db.GetHabitRow, streak int32) *client.Habit {
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
		Streak:         streak,
		Completed:      h.Completed,
		CompletedToday: h.Completed,
		CreatedAt:      h.CreatedAt.Time.Unix(),
		UpdatedAt:      h.UpdatedAt.Time.Unix(),
	}
}
