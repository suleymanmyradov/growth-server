-- Goal rows are returned with a resolved category slug and a derived
-- `completed` flag so callers never deal with category_id directly.

-- name: ListGoals :many
SELECT g.id, g.user_id, g.category_id, g.title, g.description, g.status, g.progress, g.due_date, g.created_at, g.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       (g.status = 'completed') AS completed
FROM goals g
LEFT JOIN categories c ON c.id = g.category_id
WHERE g.user_id = $1
ORDER BY g.created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetGoal :one
SELECT g.id, g.user_id, g.category_id, g.title, g.description, g.status, g.progress, g.due_date, g.created_at, g.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       (g.status = 'completed') AS completed
FROM goals g
LEFT JOIN categories c ON c.id = g.category_id
WHERE g.id = $1;

-- name: CreateGoal :one
WITH ins AS (
    INSERT INTO goals (title, description, category_id, due_date, user_id)
    VALUES ($1, $2, (SELECT c2.id FROM categories c2 WHERE c2.slug = $3), $4, $5)
    RETURNING *
)
SELECT ins.id, ins.user_id, ins.category_id, ins.title, ins.description, ins.status, ins.progress, ins.due_date, ins.created_at, ins.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       (ins.status = 'completed') AS completed
FROM ins
LEFT JOIN categories c ON c.id = ins.category_id;

-- name: UpdateGoal :one
WITH upd AS (
    UPDATE goals
    SET title = $2, description = $3,
        category_id = (SELECT c2.id FROM categories c2 WHERE c2.slug = $4),
        due_date = $5
    WHERE goals.id = $1
    RETURNING *
)
SELECT upd.id, upd.user_id, upd.category_id, upd.title, upd.description, upd.status, upd.progress, upd.due_date, upd.created_at, upd.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       (upd.status = 'completed') AS completed
FROM upd
LEFT JOIN categories c ON c.id = upd.category_id;

-- name: DeleteGoal :exec
DELETE FROM goals WHERE id = $1;

-- name: ToggleGoal :one
WITH upd AS (
    UPDATE goals
    SET status = CASE WHEN status = 'completed' THEN 'active' ELSE 'completed' END,
        progress = CASE WHEN status = 'completed' THEN 0 ELSE 100 END
    WHERE goals.id = $1
    RETURNING *
)
SELECT upd.id, upd.user_id, upd.category_id, upd.title, upd.description, upd.status, upd.progress, upd.due_date, upd.created_at, upd.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       (upd.status = 'completed') AS completed
FROM upd
LEFT JOIN categories c ON c.id = upd.category_id;

-- name: UpdateGoalProgress :one
WITH upd AS (
    UPDATE goals
    SET progress = $2,
        status = CASE WHEN $2 >= 100 THEN 'completed' ELSE status END
    WHERE goals.id = $1
    RETURNING *
)
SELECT upd.id, upd.user_id, upd.category_id, upd.title, upd.description, upd.status, upd.progress, upd.due_date, upd.created_at, upd.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       (upd.status = 'completed') AS completed
FROM upd
LEFT JOIN categories c ON c.id = upd.category_id;

-- name: GetGoalsByIDs :many
SELECT g.id, g.user_id, g.category_id, g.title, g.description, g.status, g.progress, g.due_date, g.created_at, g.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       (g.status = 'completed') AS completed
FROM goals g
LEFT JOIN categories c ON c.id = g.category_id
WHERE g.id = ANY($1::uuid[]);

-- name: CountGoalsByUser :one
SELECT COUNT(*) FROM goals WHERE user_id = $1;

-- name: ListGoalsKeyset :many
-- Keyset pagination: pass last_created_at from the previous page (or NULL).
SELECT g.id, g.user_id, g.category_id, g.title, g.description, g.status, g.progress, g.due_date, g.created_at, g.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       (g.status = 'completed') AS completed
FROM goals g
LEFT JOIN categories c ON c.id = g.category_id
WHERE g.user_id = $1
  AND ($2::timestamptz IS NULL OR g.created_at < $2)
ORDER BY g.created_at DESC
LIMIT $3;
