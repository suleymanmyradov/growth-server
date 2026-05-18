package consumer

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/suleymanmyradov/growth-server/pkg/events"
	"github.com/suleymanmyradov/growth-server/services/microservices/notifications/rpc/internal/repository"
)

func TestEventsHandler_InvalidEnvelope(t *testing.T) {
	h := NewEventsHandler(&repository.Repository{}, nil, nil)
	err := h.Consume(context.Background(), "", "not-json")
	if err != nil {
		t.Fatalf("expected nil on invalid envelope, got %v", err)
	}
}

func TestEventsHandler_InvalidEventID(t *testing.T) {
	h := NewEventsHandler(&repository.Repository{}, nil, nil)
	env := events.Envelope{EventID: "not-a-uuid", EventType: string(events.TypeCheckInCreated)}
	raw, _ := json.Marshal(env)
	err := h.Consume(context.Background(), "", string(raw))
	if err != nil {
		t.Fatalf("expected nil on invalid eventID, got %v", err)
	}
}

func TestEventsHandler_UnhandledEventType(t *testing.T) {
	h := NewEventsHandler(&repository.Repository{}, nil, nil)
	env, _ := events.NewEnvelope("unknown_type", nil)
	raw, _ := json.Marshal(env)
	err := h.Consume(context.Background(), "", string(raw))
	if err != nil {
		t.Fatalf("expected nil on unhandled type, got %v", err)
	}
}

func TestEventsHandler_MalformedPayload(t *testing.T) {
	h := NewEventsHandler(&repository.Repository{}, nil, nil)
	env, _ := events.NewEnvelope(events.TypeCheckInCreated, json.RawMessage(`{bad`))
	raw, _ := json.Marshal(env)
	err := h.Consume(context.Background(), "", string(raw))
	if err != nil {
		t.Fatalf("expected nil on malformed payload, got %v", err)
	}
}

func TestReminderDueHandler_InvalidEnvelope(t *testing.T) {
	h := NewReminderDueHandler(&repository.Repository{}, nil)
	err := h.Consume(context.Background(), "", "{bad")
	if err != nil {
		t.Fatalf("expected nil on invalid envelope, got %v", err)
	}
}

func TestReminderDueHandler_InvalidEventID(t *testing.T) {
	h := NewReminderDueHandler(&repository.Repository{}, nil)
	env := events.Envelope{EventID: "bad", EventType: string(events.TypeReminderDue)}
	raw, _ := json.Marshal(env)
	err := h.Consume(context.Background(), "", string(raw))
	if err != nil {
		t.Fatalf("expected nil on invalid eventID, got %v", err)
	}
}

func TestReminderDueHandler_UnhandledType(t *testing.T) {
	h := NewReminderDueHandler(&repository.Repository{}, nil)
	env, _ := events.NewEnvelope(events.TypeReminderDue, events.ReminderDue{
		ReminderID: uuid.New().String(),
		UserID:     uuid.New().String(),
		Type:       "unknown_reminder",
	})
	raw, _ := json.Marshal(env)
	err := h.Consume(context.Background(), "", string(raw))
	if err != nil {
		t.Fatalf("expected nil on unhandled type, got %v", err)
	}
}
