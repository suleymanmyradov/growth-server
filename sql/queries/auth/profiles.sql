-- name: CreateProfile :one
INSERT INTO profiles (user_id, bio, location, website, interests, avatar_url)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id, user_id, bio, location, website, interests, avatar_url, created_at, updated_at;

-- name: GetProfileByUserID :one
SELECT id, user_id, bio, location, website, interests, avatar_url, created_at, updated_at
FROM profiles
WHERE user_id = $1;

-- name: UpdateProfile :one
UPDATE profiles
SET bio = $2,
    location = $3,
    website = $4,
    interests = $5,
    avatar_url = $6,
    updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1
RETURNING id, user_id, bio, location, website, interests, avatar_url, created_at, updated_at;
