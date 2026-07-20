-- name: ListHabitTemplates :many
SELECT
    ht.id, ht.name, ht.description, ht.sort_order, ht.is_active,
    ht.created_at, ht.updated_at,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug
FROM habit_templates ht
LEFT JOIN categories c ON ht.category_id = c.id
WHERE ht.is_active = true
ORDER BY ht.sort_order ASC, ht.created_at ASC;

-- name: ListGoalTemplates :many
SELECT
    gt.id, gt.title, gt.description, gt.sort_order, gt.is_active,
    gt.created_at, gt.updated_at,
    c.id AS category_id, c.name AS category_name, c.slug AS category_slug
FROM goal_templates gt
LEFT JOIN categories c ON gt.category_id = c.id
WHERE gt.is_active = true
ORDER BY gt.sort_order ASC, gt.created_at ASC;
