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
		Status:  db.CheckInStatus(status),
	}
	if mood != "" {
		m := db.MoodType(mood)
		params.Mood = &m
	}
	if energy != "" {
		e := db.EnergyLevel(energy)
		params.Energy = &e
	}
	if blocker != "" {
		b := db.BlockerType(blocker)
		params.Blocker = &b
	}
	if note != "" {
		params.Note = &note
	}
	return params
}

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
