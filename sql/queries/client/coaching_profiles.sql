-- Coaching preferences live in their own table (owned by ai-coach).

-- name: GetCoachingProfile :one
SELECT user_id, accountability_style, coach_tone AS preferred_tone,
       difficulty AS difficulty_preference, primary_motivation,
       common_blockers, coaching_notes, last_context_refresh_at,
       created_at, updated_at
FROM coaching_profiles
WHERE user_id = $1;

-- name: UpsertCoachingProfile :one
INSERT INTO coaching_profiles (
    user_id, accountability_style, coach_tone, difficulty,
    primary_motivation, common_blockers, coaching_notes, last_context_refresh_at
)
VALUES ($1, $2, $3, $4, $5, $6, $7, now())
ON CONFLICT (user_id)
DO UPDATE SET
    accountability_style = EXCLUDED.accountability_style,
    coach_tone = EXCLUDED.coach_tone,
    difficulty = EXCLUDED.difficulty,
    primary_motivation = EXCLUDED.primary_motivation,
    common_blockers = EXCLUDED.common_blockers,
    coaching_notes = EXCLUDED.coaching_notes,
    last_context_refresh_at = now()
RETURNING user_id, accountability_style, coach_tone AS preferred_tone,
          difficulty AS difficulty_preference, primary_motivation,
          common_blockers, coaching_notes, last_context_refresh_at,
          created_at, updated_at;

-- name: UpdateCoachingProfilePreferences :one
INSERT INTO coaching_profiles (user_id, accountability_style, coach_tone, difficulty)
VALUES ($1, $2, $3, $4)
ON CONFLICT (user_id)
DO UPDATE SET
    accountability_style = EXCLUDED.accountability_style,
    coach_tone = EXCLUDED.coach_tone,
    difficulty = EXCLUDED.difficulty
RETURNING user_id, accountability_style, coach_tone AS preferred_tone,
          difficulty AS difficulty_preference, primary_motivation,
          common_blockers, coaching_notes, last_context_refresh_at,
          created_at, updated_at;

-- name: UpdateCoachingProfileBlockers :one
UPDATE coaching_profiles
SET common_blockers = $2
WHERE user_id = $1
RETURNING user_id, accountability_style, coach_tone AS preferred_tone,
          difficulty AS difficulty_preference, primary_motivation,
          common_blockers, coaching_notes, last_context_refresh_at,
          created_at, updated_at;

-- name: UpdateCoachingProfileNotes :one
UPDATE coaching_profiles
SET coaching_notes = $2
WHERE user_id = $1
RETURNING user_id, accountability_style, coach_tone AS preferred_tone,
          difficulty AS difficulty_preference, primary_motivation,
          common_blockers, coaching_notes, last_context_refresh_at,
          created_at, updated_at;

-- name: UpdateCoachingProfileContextRefresh :one
UPDATE coaching_profiles
SET last_context_refresh_at = now()
WHERE user_id = $1
RETURNING user_id, accountability_style, coach_tone AS preferred_tone,
          difficulty AS difficulty_preference, primary_motivation,
          common_blockers, coaching_notes, last_context_refresh_at,
          created_at, updated_at;

-- name: DeleteCoachingProfile :exec
DELETE FROM coaching_profiles WHERE user_id = $1;
