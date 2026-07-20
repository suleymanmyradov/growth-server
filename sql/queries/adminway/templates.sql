-- name: AdminListHabitTemplates :many
SELECT
    ht.id, ht.name, ht.description, ht.category_id, ht.sort_order, ht.is_active,
    ht.created_at, ht.updated_at,
    c.id AS category_id_joined, c.name AS category_name, c.slug AS category_slug
FROM habit_templates ht
LEFT JOIN categories c ON ht.category_id = c.id
ORDER BY ht.sort_order ASC, ht.created_at ASC;

-- name: AdminGetHabitTemplate :one
SELECT
    ht.id, ht.name, ht.description, ht.category_id, ht.sort_order, ht.is_active,
    ht.created_at, ht.updated_at,
    c.id AS category_id_joined, c.name AS category_name, c.slug AS category_slug
FROM habit_templates ht
LEFT JOIN categories c ON ht.category_id = c.id
WHERE ht.id = $1;

-- name: AdminCreateHabitTemplate :one
INSERT INTO habit_templates (name, description, category_id, sort_order, is_active)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, name, description, category_id, sort_order, is_active, created_at, updated_at;

-- name: AdminUpdateHabitTemplate :one
UPDATE habit_templates
SET name = $2, description = $3, category_id = $4, sort_order = $5, is_active = $6
WHERE id = $1
RETURNING id, name, description, category_id, sort_order, is_active, created_at, updated_at;

-- name: AdminDeleteHabitTemplate :exec
DELETE FROM habit_templates WHERE id = $1;

-- name: AdminListGoalTemplates :many
SELECT
    gt.id, gt.title, gt.description, gt.category_id, gt.sort_order, gt.is_active,
    gt.created_at, gt.updated_at,
    c.id AS category_id_joined, c.name AS category_name, c.slug AS category_slug
FROM goal_templates gt
LEFT JOIN categories c ON gt.category_id = c.id
ORDER BY gt.sort_order ASC, gt.created_at ASC;

-- name: AdminGetGoalTemplate :one
SELECT
    gt.id, gt.title, gt.description, gt.category_id, gt.sort_order, gt.is_active,
    gt.created_at, gt.updated_at,
    c.id AS category_id_joined, c.name AS category_name, c.slug AS category_slug
FROM goal_templates gt
LEFT JOIN categories c ON gt.category_id = c.id
WHERE gt.id = $1;

-- name: AdminCreateGoalTemplate :one
INSERT INTO goal_templates (title, description, category_id, sort_order, is_active)
VALUES ($1, $2, $3, $4, $5)
RETURNING id, title, description, category_id, sort_order, is_active, created_at, updated_at;

-- name: AdminUpdateGoalTemplate :one
UPDATE goal_templates
SET title = $2, description = $3, category_id = $4, sort_order = $5, is_active = $6
WHERE id = $1
RETURNING id, title, description, category_id, sort_order, is_active, created_at, updated_at;

-- name: AdminDeleteGoalTemplate :exec
DELETE FROM goal_templates WHERE id = $1;
