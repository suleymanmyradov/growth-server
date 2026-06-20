-- name: CreateTag :one
INSERT INTO tags (name, slug)
VALUES ($1, $2)
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name
RETURNING id, name, slug, created_at;

-- name: GetTag :one
SELECT id, name, slug, created_at FROM tags WHERE id = $1;

-- name: GetTagBySlug :one
SELECT id, name, slug, created_at FROM tags WHERE slug = $1;

-- name: UpdateTag :one
UPDATE tags
SET name = $2, slug = $3
WHERE id = $1
RETURNING id, name, slug, created_at;

-- name: DeleteTag :exec
DELETE FROM tags WHERE id = $1;

-- name: CountTagUsage :one
SELECT COUNT(*) FROM article_tags WHERE tag_id = $1;
