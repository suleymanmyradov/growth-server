-- Coaching preferences live on user_settings now (no separate profile table).
-- These queries keep the coaching-profile shape the AI coach expects.

-- name: GetCoachingProfile :one
SELECT user_id, accountability_style, coach_tone AS preferred_tone,
       difficulty AS difficulty_preference, primary_motivation,
       common_blockers, coaching_notes, last_context_refresh_at,
       created_at, updated_at
FROM user_settings
WHERE user_id = $1;

-- name: UpsertCoachingProfile :one
INSERT INTO user_settings (
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
INSERT INTO user_settings (user_id, accountability_style, coach_tone, difficulty)
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
UPDATE user_settings
SET common_blockers = $2
WHERE user_id = $1
RETURNING user_id, accountability_style, coach_tone AS preferred_tone,
          difficulty AS difficulty_preference, primary_motivation,
          common_blockers, coaching_notes, last_context_refresh_at,
          created_at, updated_at;

-- name: UpdateCoachingProfileNotes :one
UPDATE user_settings
SET coaching_notes = $2
WHERE user_id = $1
RETURNING user_id, accountability_style, coach_tone AS preferred_tone,
          difficulty AS difficulty_preference, primary_motivation,
          common_blockers, coaching_notes, last_context_refresh_at,
          created_at, updated_at;

-- name: UpdateCoachingProfileContextRefresh :one
UPDATE user_settings
SET last_context_refresh_at = now()
WHERE user_id = $1
RETURNING user_id, accountability_style, coach_tone AS preferred_tone,
          difficulty AS difficulty_preference, primary_motivation,
          common_blockers, coaching_notes, last_context_refresh_at,
          created_at, updated_at;

-- name: DeleteCoachingProfile :exec
-- Reset coaching fields to their defaults (settings row itself stays).
UPDATE user_settings
SET accountability_style = 'balanced',
    coach_tone = 'supportive',
    difficulty = 'adaptive',
    primary_motivation = NULL,
    common_blockers = '[]'::jsonb,
    coaching_notes = '{}'::jsonb,
    last_context_refresh_at = NULL
WHERE user_id = $1;
