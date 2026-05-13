-- name: ListCategories :many
SELECT id, name, slug, entity_type, sort_order, created_at, updated_at FROM categories
WHERE entity_type = $1
ORDER BY sort_order ASC;

-- name: ListAllCategories :many
SELECT id, name, slug, entity_type, sort_order, created_at, updated_at FROM categories
ORDER BY entity_type, sort_order ASC;

-- name: GetCategory :one
SELECT id, name, slug, entity_type, sort_order, created_at, updated_at FROM categories WHERE id = $1;

-- name: GetCategoryBySlug :one
SELECT id, name, slug, entity_type, sort_order, created_at, updated_at FROM categories WHERE slug = $1 AND entity_type = $2;

-- name: CreateCategory :one
INSERT INTO categories (name, slug, entity_type, sort_order)
VALUES ($1, $2, $3, $4)
RETURNING id, name, slug, entity_type, sort_order, created_at, updated_at;

-- name: UpdateCategory :one
UPDATE categories
SET name = $2, slug = $3, sort_order = $4, updated_at = CURRENT_TIMESTAMP
WHERE id = $1
RETURNING id, name, slug, entity_type, sort_order, created_at, updated_at;

-- name: DeleteCategory :exec
DELETE FROM categories WHERE id = $1;

-- name: CountCategoriesByType :one
SELECT COUNT(*) FROM categories WHERE entity_type = $1;
