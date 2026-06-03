package events

import (
	"encoding/json"
	"fmt"
)

// ValidEventType returns true for known event types.
func ValidEventType(t EventType) bool {
	switch t {
	case TypeCheckInCreated,
		TypeUserOnboarded,
		TypeSettingsChanged,
		TypeReminderDue,
		TypeCheckInFeedbackGenerated:
		return true
	}
	return false
}

// CurrentVersion returns the current schema version for an event type.
// All event types are currently at version 1.
func CurrentVersion(t EventType) int {
	if ValidEventType(t) {
		return 1
	}
	return 0
}

// Validate checks that the envelope carries a known event type and a
// supported version. It does NOT validate payload shape.
func (e Envelope) Validate() error {
	et := EventType(e.EventType)
	if !ValidEventType(et) {
		return fmt.Errorf("unknown event type %q", e.EventType)
	}
	current := CurrentVersion(et)
	if e.Version < 1 || e.Version > current {
		return fmt.Errorf("unsupported version %d for %q (current=%d)", e.Version, e.EventType, current)
	}
	if len(e.Payload) == 0 {
		return fmt.Errorf("payload is empty")
	}
	if !json.Valid(e.Payload) {
		return fmt.Errorf("payload is not valid JSON")
	}
	return nil
}
