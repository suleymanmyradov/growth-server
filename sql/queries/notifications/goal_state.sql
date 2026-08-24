-- Local goal read model for the notifications service, maintained from
-- goal_created/goal_updated/goal_deleted/goal_completed events.

-- name: UpsertGoalState :one
INSERT INTO notification_goal_state (goal_id, user_id, title, deadline, completed)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (goal_id) DO UPDATE SET
    title     = EXCLUDED.title,
    deadline  = EXCLUDED.deadline,
    completed = EXCLUDED.completed,
    updated_at = now()
RETURNING goal_id, user_id, title, deadline, completed, created_at, updated_at;

-- name: DeleteGoalState :execrows
DELETE FROM notification_goal_state WHERE goal_id = $1;

-- name: MarkGoalCompleted :execrows
UPDATE notification_goal_state
SET completed = true, updated_at = now()
WHERE goal_id = $1;

-- name: ListUpcomingGoalDeadlines :many
-- Returns active (not completed) goals with future deadlines for a user,
-- ordered by deadline. Used to reschedule goal_deadline reminders when the
-- user re-enables the goalReminders preference.
SELECT goal_id, user_id, title, deadline, completed, created_at, updated_at
FROM notification_goal_state
WHERE user_id = $1
  AND NOT completed
  AND deadline IS NOT NULL
  AND deadline > $2
ORDER BY deadline;

-- name: DeleteGoalStateByUser :exec
DELETE FROM notification_goal_state WHERE user_id = $1;
