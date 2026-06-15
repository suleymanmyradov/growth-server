-- name: ListCategories :many
SELECT * FROM categories ORDER BY sort_order ASC;

-- name: GetCategory :one
SELECT * FROM categories WHERE id = $1;

-- name: GetCategoryBySlug :one
SELECT * FROM categories WHERE slug = $1;

-- name: CreateCategory :one
INSERT INTO categories (name, slug, sort_order)
VALUES ($1, $2, $3)
RETURNING *;

-- name: UpdateCategory :one
UPDATE categories
SET name = $2, slug = $3, sort_order = $4
WHERE id = $1
RETURNING *;

-- name: DeleteCategory :exec
DELETE FROM categories WHERE id = $1;

-- name: CountCategories :one
SELECT COUNT(*) FROM categories;
