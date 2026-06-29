-- name: GetUserProfileByID :one
SELECT id, username, full_name, bio, location, website, interests, avatar_url
FROM users
WHERE id = $1;
