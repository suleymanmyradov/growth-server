-- name: ListCurrentUserFacts :many
-- The prompt-injection path: current, sufficiently-confident facts for one
-- user. Ordered so user-authored facts and high-confidence facts win the
-- budget when the caller truncates.
SELECT id, user_id, fact, category, confidence, source_message_id, user_authored,
       superseded_by, superseded_at, created_at, updated_at
FROM user_facts
WHERE user_id = $1
  AND superseded_by IS NULL
  AND (confidence >= $2 OR user_authored)
ORDER BY user_authored DESC, confidence DESC, created_at DESC
LIMIT $3;

-- name: ListAllUserFacts :many
-- The user-facing "what do you remember about me" view: every current fact
-- regardless of confidence, so users can see and correct low-confidence
-- guesses rather than being surprised by them later.
SELECT id, user_id, fact, category, confidence, source_message_id, user_authored,
       superseded_by, superseded_at, created_at, updated_at
FROM user_facts
WHERE user_id = $1 AND superseded_by IS NULL
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetUserFact :one
SELECT id, user_id, fact, category, confidence, source_message_id, user_authored,
       superseded_by, superseded_at, created_at, updated_at
FROM user_facts
WHERE id = $1 AND user_id = $2;

-- name: CreateUserFact :one
-- ON CONFLICT DO NOTHING against the live-uniqueness index: the extractor runs
-- every turn and will keep proposing facts it already recorded. Returning no
-- row on conflict is the correct signal for "already known" -- the caller
-- treats it as a no-op rather than an error.
INSERT INTO user_facts (user_id, fact, category, confidence, source_message_id, user_authored)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING
RETURNING id, user_id, fact, category, confidence, source_message_id, user_authored,
          superseded_by, superseded_at, created_at, updated_at;

-- name: SupersedeUserFact :one
-- Point an old fact at the one that replaces it. Scoped by user_id so one user
-- can never rewrite another's memory, and guarded on superseded_by IS NULL so a
-- double-apply cannot re-link an already-superseded row.
UPDATE user_facts
SET superseded_by = $3, superseded_at = now()
WHERE id = $1 AND user_id = $2 AND superseded_by IS NULL
RETURNING id, user_id, fact, category, confidence, source_message_id, user_authored,
          superseded_by, superseded_at, created_at, updated_at;

-- name: ForgetUserFact :exec
-- Hard delete, for "forget this about me". Deliberately not a supersession:
-- when a user asks the coach to forget something, retaining it as history
-- defeats the request.
DELETE FROM user_facts WHERE id = $1 AND user_id = $2;

-- name: ForgetAllUserFacts :exec
-- Backs "disable long-term memory" and account deletion.
DELETE FROM user_facts WHERE user_id = $1;

-- name: CountCurrentUserFacts :one
SELECT count(*) FROM user_facts
WHERE user_id = $1 AND superseded_by IS NULL;
