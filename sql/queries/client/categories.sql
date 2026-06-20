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

-- name: CountArticlesByCategory :one
SELECT COUNT(*) FROM articles WHERE category_id = $1;

-- name: ReorderCategories :exec
UPDATE categories
SET sort_order = new_sort.sort_order
FROM (SELECT unnest($1::uuid[]) AS id, unnest($2::int[]) AS sort_order) AS new_sort
WHERE categories.id = new_sort.id;
