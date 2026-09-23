-- name: ListCategories :many
SELECT id, name, slug, sort_order, created_at, updated_at
FROM categories
ORDER BY sort_order ASC;

-- name: GetCategory :one
SELECT id, name, slug, sort_order, created_at, updated_at
FROM categories
WHERE id = $1;

-- name: GetCategoryBySlug :one
SELECT id, name, slug, sort_order, created_at, updated_at
FROM categories
WHERE slug = $1;

-- name: CreateCategory :one
INSERT INTO categories (name, slug, sort_order)
VALUES ($1, $2, $3)
RETURNING id, name, slug, sort_order, created_at, updated_at;

-- name: UpdateCategory :one
-- sort_order is a plain int in the contract, so an omitted field arrives as 0
-- and must preserve the stored order rather than resetting the category to
-- the top of the list.
UPDATE categories
SET name = $2, slug = $3,
    sort_order = CASE WHEN $4 = 0 THEN categories.sort_order ELSE $4 END
WHERE id = $1
RETURNING id, name, slug, sort_order, created_at, updated_at;

-- name: DeleteCategory :exec
DELETE FROM categories WHERE id = $1;

-- name: CountCategories :one
SELECT COUNT(*) FROM categories;

-- name: GetCategoriesByIDs :many
SELECT id, name, slug, sort_order, created_at, updated_at
FROM categories
WHERE id = ANY($1::uuid[]);

-- name: ReorderCategories :exec
UPDATE categories
SET sort_order = new_sort.sort_order
FROM (SELECT unnest($1::uuid[]) AS id, unnest($2::int[]) AS sort_order) AS new_sort
WHERE categories.id = new_sort.id;
