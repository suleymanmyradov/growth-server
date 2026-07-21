// Package notifications defines shared notification type constants and
// validation helpers. It is the single source of truth for the set of
// notification types accepted by the notifications service, mirroring the
// notifications.type CHECK constraint in the database (see migration 016 +
// migration 040 which adds ai_feedback).
//
// Both adminway (admin broadcast validation) and the notifications consumer
// (broadcast ingestion) import this package so they cannot drift apart.
package notifications

// Type is a notification type string stored in notifications.type.
type Type string

const (
	TypeHabitReminder  Type = "habit_reminder"
	TypeMissedCheckIn  Type = "missed_check_in"
	TypeGoalDeadline   Type = "goal_deadline"
	TypeAchievement    Type = "achievement"
	TypeWeeklyReview   Type = "weekly_review"
	TypeEncouragement  Type = "encouragement"
	TypeSystem         Type = "system"
	TypeAIFeedback     Type = "ai_feedback"
)

// allTypes lists every type allowed by the notifications.type CHECK
// constraint. Keep in sync with sql/migrations_v2/016_notifications.up.sql
// and 040_notifications_ai_feedback_type.up.sql.
var allTypes = map[Type]bool{
	TypeHabitReminder: true,
	TypeMissedCheckIn: true,
	TypeGoalDeadline:  true,
	TypeAchievement:   true,
	TypeWeeklyReview:  true,
	TypeEncouragement: true,
	TypeSystem:        true,
	TypeAIFeedback:    true,
}

// All returns every supported notification type, in canonical order.
func All() []Type {
	return []Type{
		TypeHabitReminder,
		TypeMissedCheckIn,
		TypeGoalDeadline,
		TypeAchievement,
		TypeWeeklyReview,
		TypeEncouragement,
		TypeSystem,
		TypeAIFeedback,
	}
}

// IsValid reports whether t is a notification type accepted by the
// notifications service and the notifications.type CHECK constraint.
func IsValid(t string) bool {
	return allTypes[Type(t)]
}
