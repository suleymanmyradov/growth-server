-- name: GetCoachingProfile :one
SELECT * FROM user_coaching_profiles
WHERE user_id = $1;

-- name: UpsertCoachingProfile :one
INSERT INTO user_coaching_profiles (
    user_id,
    accountability_style,
    preferred_tone,
    difficulty_preference,
    primary_motivation,
    common_blockers,
    coaching_notes,
    last_context_refresh_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, CURRENT_TIMESTAMP)
ON CONFLICT (user_id)
DO UPDATE SET
    accountability_style = EXCLUDED.accountability_style,
    preferred_tone = EXCLUDED.preferred_tone,
    difficulty_preference = EXCLUDED.difficulty_preference,
    primary_motivation = EXCLUDED.primary_motivation,
    common_blockers = EXCLUDED.common_blockers,
    coaching_notes = EXCLUDED.coaching_notes,
    last_context_refresh_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: UpdateCoachingProfilePreferences :one
INSERT INTO user_coaching_profiles (
    user_id,
    accountability_style,
    preferred_tone,
    difficulty_preference,
    common_blockers,
    coaching_notes,
    last_context_refresh_at
)
VALUES ($1, $2, $3, $4, '[]'::jsonb, '{}'::jsonb, CURRENT_TIMESTAMP)
ON CONFLICT (user_id)
DO UPDATE SET
    accountability_style = EXCLUDED.accountability_style,
    preferred_tone = EXCLUDED.preferred_tone,
    difficulty_preference = EXCLUDED.difficulty_preference,
    updated_at = CURRENT_TIMESTAMP
RETURNING *;

-- name: UpdateCoachingProfileBlockers :one
UPDATE user_coaching_profiles
SET
    common_blockers = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1
RETURNING *;

-- name: UpdateCoachingProfileNotes :one
UPDATE user_coaching_profiles
SET
    coaching_notes = $2,
    updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1
RETURNING *;

-- name: UpdateCoachingProfileContextRefresh :one
UPDATE user_coaching_profiles
SET
    last_context_refresh_at = CURRENT_TIMESTAMP,
    updated_at = CURRENT_TIMESTAMP
WHERE user_id = $1
RETURNING *;

-- name: DeleteCoachingProfile :exec
DELETE FROM user_coaching_profiles WHERE user_id = $1;