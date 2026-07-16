-- Event-fed read model for user profiles (V3).

-- name: UpsertUserProfile :exec
INSERT INTO user_profiles (id, username, full_name, bio, location, website, interests, avatar_url)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (id) DO UPDATE SET
    username   = EXCLUDED.username,
    full_name  = EXCLUDED.full_name,
    bio        = EXCLUDED.bio,
    location   = EXCLUDED.location,
    website    = EXCLUDED.website,
    interests  = EXCLUDED.interests,
    avatar_url = EXCLUDED.avatar_url,
    updated_at = now();

-- name: DeleteUserProfile :exec
DELETE FROM user_profiles WHERE id = $1;
