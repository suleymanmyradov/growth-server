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
	h := NewEventsHandler(&repository.Repository{}, nil, nil, nil, nil)
	err := h.Consume(context.Background(), "", "not-json")
	if err != nil {
		t.Fatalf("expected nil on invalid envelope, got %v", err)
	}
}

func TestEventsHandler_InvalidEventID(t *testing.T) {
	h := NewEventsHandler(&repository.Repository{}, nil, nil, nil, nil)
	env := events.Envelope{EventID: "not-a-uuid", EventType: string(events.TypeCheckInCreated)}
	raw, _ := json.Marshal(env)
	err := h.Consume(context.Background(), "", string(raw))
	if err != nil {
		t.Fatalf("expected nil on invalid eventID, got %v", err)
	}
}

func TestEventsHandler_UnhandledEventType(t *testing.T) {
	h := NewEventsHandler(&repository.Repository{}, nil, nil, nil, nil)
	env, _ := events.NewEnvelope("unknown_type", nil)
	raw, _ := json.Marshal(env)
	err := h.Consume(context.Background(), "", string(raw))
	if err != nil {
		t.Fatalf("expected nil on unhandled type, got %v", err)
	}
}

func TestEventsHandler_MalformedPayload(t *testing.T) {
	h := NewEventsHandler(&repository.Repository{}, nil, nil, nil, nil)
	env, _ := events.NewEnvelope(events.TypeCheckInCreated, json.RawMessage(`{bad`))
	raw, _ := json.Marshal(env)
	err := h.Consume(context.Background(), "", string(raw))
	if err != nil {
		t.Fatalf("expected nil on malformed payload, got %v", err)
	}
}

func TestReminderDueHandler_InvalidEnvelope(t *testing.T) {
	h := NewReminderDueHandler(&repository.Repository{}, nil, nil, nil, nil, nil)
	err := h.Consume(context.Background(), "", "{bad")
	if err != nil {
		t.Fatalf("expected nil on invalid envelope, got %v", err)
	}
}

func TestReminderDueHandler_InvalidEventID(t *testing.T) {
	h := NewReminderDueHandler(&repository.Repository{}, nil, nil, nil, nil, nil)
	env := events.Envelope{EventID: "bad", EventType: string(events.TypeReminderDue)}
	raw, _ := json.Marshal(env)
	err := h.Consume(context.Background(), "", string(raw))
	if err != nil {
		t.Fatalf("expected nil on invalid eventID, got %v", err)
	}
}

func TestReminderDueHandler_UnhandledType(t *testing.T) {
	h := NewReminderDueHandler(&repository.Repository{}, nil, nil, nil, nil, nil)
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

// ---- DLQ tests ----

// fakeDLQ captures DLQ messages for assertions in tests.
type fakeDLQ struct {
	messages []events.DLQMessage
	err      error
}

func (f *fakeDLQ) Publish(_ context.Context, msg events.DLQMessage) error {
	f.messages = append(f.messages, msg)
	return f.err
}

func TestEventsHandler_InvalidEnvelope_RoutedToDLQ(t *testing.T) {
	dlq := &fakeDLQ{}
	h := NewEventsHandler(&repository.Repository{}, nil, nil, nil, dlq)

	err := h.Consume(context.Background(), "", "not-json")
	if err != nil {
		t.Fatalf("expected nil (DLQ absorbs the error), got %v", err)
	}
	if len(dlq.messages) != 1 {
		t.Fatalf("expected 1 DLQ message, got %d", len(dlq.messages))
	}
	if dlq.messages[0].Reason == "" {
		t.Error("expected non-empty reason in DLQ message")
	}
	if !dlq.messages[0].Permanent {
		t.Error("expected poison message to be marked permanent")
	}
	if dlq.messages[0].Raw != "not-json" {
		t.Errorf("expected raw payload in DLQ message, got %q", dlq.messages[0].Raw)
	}
}

func TestEventsHandler_InvalidEventID_RoutedToDLQ(t *testing.T) {
	dlq := &fakeDLQ{}
	h := NewEventsHandler(&repository.Repository{}, nil, nil, nil, dlq)

	env := events.Envelope{EventID: "not-a-uuid", EventType: string(events.TypeCheckInCreated)}
	raw, _ := json.Marshal(env)

	err := h.Consume(context.Background(), "", string(raw))
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(dlq.messages) != 1 {
		t.Fatalf("expected 1 DLQ message, got %d", len(dlq.messages))
	}
	if dlq.messages[0].Original.EventID != "not-a-uuid" {
		t.Errorf("expected original envelope preserved in DLQ, got %+v", dlq.messages[0].Original)
	}
}

func TestReminderDueHandler_InvalidEnvelope_RoutedToDLQ(t *testing.T) {
	dlq := &fakeDLQ{}
	h := NewReminderDueHandler(&repository.Repository{}, nil, nil, dlq, nil, nil)

	err := h.Consume(context.Background(), "", "{bad")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
	if len(dlq.messages) != 1 {
		t.Fatalf("expected 1 DLQ message, got %d", len(dlq.messages))
	}
}
