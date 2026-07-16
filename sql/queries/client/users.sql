-- name: GetUserProfileByID :one
SELECT id, username, full_name, bio, location, website, interests, avatar_url
FROM user_profiles
WHERE id = $1;
