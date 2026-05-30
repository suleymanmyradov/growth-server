-- name: ListGoals :many
SELECT *
FROM goals
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetGoal :one
SELECT *
FROM goals
WHERE id = $1;

-- name: CreateGoal :one
INSERT INTO goals (title, description, category, due_date, user_id)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: UpdateGoal :one
UPDATE goals
SET title = $2, description = $3, category = $4, due_date = $5, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: DeleteGoal :exec
DELETE FROM goals WHERE id = $1;

-- name: ToggleGoal :one
UPDATE goals
SET status = CASE WHEN status = 'completed' THEN 'active'::goal_status_type ELSE 'completed'::goal_status_type END,
    progress = CASE WHEN status = 'completed' THEN 0 ELSE 100 END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: UpdateGoalProgress :one
UPDATE goals
SET progress = $2,
    status = CASE WHEN $2 >= 100 THEN 'completed'::goal_status_type ELSE status END,
    updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING *;

-- name: GetGoalsByIDs :many
-- Bulk lookup for goal list views (e.g. resolving saved goals).
SELECT *
FROM goals
WHERE id = ANY($1::uuid[]);

-- name: CountGoalsByUser :one
SELECT COUNT(*) FROM goals WHERE user_id = $1;
