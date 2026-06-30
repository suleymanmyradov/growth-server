package conversationservicelogic

import (
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/internal/repository/db"
	"github.com/suleymanmyradov/growth-server/services/microservices/ai-coach/rpc/pb/aicoach"
)

func protoConversation(c db.Conversation) *aicoach.Conversation {
	return &aicoach.Conversation{
		Id:          c.ID.String(),
		UserId:      c.UserID.String(),
		Title:       c.Title,
		Type:        c.Type,
		LastMessage: c.LastMessage,
		CreatedAt:   tsToMillis(c.CreatedAt),
		UpdatedAt:   tsToMillis(c.UpdatedAt),
		Archived:    c.Archived,
	}
}

func protoMessage(m db.ConversationMessage) *aicoach.ConversationMessage {
	return &aicoach.ConversationMessage{
		Id:             m.ID.String(),
		ConversationId: m.ConversationID.String(),
		Role:           m.Role,
		Content:        m.Content,
		CreatedAt:      tsToMillis(m.CreatedAt),
	}
}

func parseUUID(s string) (uuid.UUID, error) {
	return uuid.Parse(s)
}

func tsToMillis(ts pgtype.Timestamptz) int64 {
	if !ts.Valid {
		return 0
	}
	return ts.Time.Unix()
}
