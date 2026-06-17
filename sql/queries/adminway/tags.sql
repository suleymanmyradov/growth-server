-- name: ListTags :many
SELECT id, name, slug
FROM tags
ORDER BY name;
