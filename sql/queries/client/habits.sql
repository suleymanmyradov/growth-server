-- Habit rows carry a resolved category slug and a derived `completed` flag:
-- completed = a 'completed' check-in exists for the habit today (in the
-- owner's timezone). There is no stored boolean to keep in sync. The streak
-- is also derived (see GetHabitStreak/GetHabitStreaks); there is no stored
-- streak column.

-- name: ListHabits :many
SELECT h.id, h.user_id, h.category_id, h.name, h.description, h.created_at, h.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       EXISTS (
           SELECT 1 FROM check_ins ci
           WHERE ci.habit_id = h.id AND ci.status = 'completed'
             AND ci.local_date = (now() AT TIME ZONE COALESCE(
                 (SELECT s.timezone FROM user_settings s WHERE s.user_id = h.user_id), 'UTC'))::date
       ) AS completed
FROM habits h
LEFT JOIN categories c ON c.id = h.category_id
WHERE h.user_id = $1
ORDER BY h.created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetHabit :one
SELECT h.id, h.user_id, h.category_id, h.name, h.description, h.created_at, h.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       EXISTS (
           SELECT 1 FROM check_ins ci
           WHERE ci.habit_id = h.id AND ci.status = 'completed'
             AND ci.local_date = (now() AT TIME ZONE COALESCE(
                 (SELECT s.timezone FROM user_settings s WHERE s.user_id = h.user_id), 'UTC'))::date
       ) AS completed
FROM habits h
LEFT JOIN categories c ON c.id = h.category_id
WHERE h.id = $1;

-- name: CreateHabit :one
WITH ins AS (
    INSERT INTO habits (name, description, category_id, user_id)
    VALUES ($1, $2, (SELECT c2.id FROM categories c2 WHERE c2.slug = $3), $4)
    RETURNING id, user_id, category_id, name, description, created_at, updated_at
)
SELECT ins.id, ins.user_id, ins.category_id, ins.name, ins.description, ins.created_at, ins.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       false AS completed
FROM ins
LEFT JOIN categories c ON c.id = ins.category_id;

-- name: UpdateHabit :one
WITH upd AS (
    UPDATE habits
    SET name = $2, description = $3,
        category_id = (SELECT c2.id FROM categories c2 WHERE c2.slug = $4)
    WHERE habits.id = $1
    RETURNING id, user_id, category_id, name, description, created_at, updated_at
)
SELECT upd.id, upd.user_id, upd.category_id, upd.name, upd.description, upd.created_at, upd.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       EXISTS (
           SELECT 1 FROM check_ins ci
           WHERE ci.habit_id = upd.id AND ci.status = 'completed'
             AND ci.local_date = (now() AT TIME ZONE COALESCE(
                 (SELECT s.timezone FROM user_settings s WHERE s.user_id = upd.user_id), 'UTC'))::date
       ) AS completed
FROM upd
LEFT JOIN categories c ON c.id = upd.category_id;

-- name: DeleteHabit :exec
DELETE FROM habits WHERE id = $1;

-- name: GetHabitStreaks :many
-- Computes the current streak for every habit owned by a user. The streak is
-- the number of consecutive completed days ending today OR yesterday (in the
-- owner's timezone). If the most recent completed day is older than yesterday
-- (or there are no completions), the streak is 0. The streak is derived from
-- check_ins history rather than stored on the habit, so it is always truthful
-- and never needs to be mutated by completion/reset flows.
WITH user_tz AS (
    SELECT COALESCE(s.timezone, 'UTC') AS tz FROM user_settings s WHERE s.user_id = $1
), today AS (
    SELECT (NOW() AT TIME ZONE COALESCE((SELECT tz FROM user_tz), 'UTC'))::date AS d
), completed AS (
    SELECT ci.habit_id, ci.local_date
    FROM check_ins ci
    WHERE ci.user_id = $1 AND ci.status = 'completed'
), islands AS (
    SELECT habit_id, local_date,
           local_date - (ROW_NUMBER() OVER (PARTITION BY habit_id ORDER BY local_date))::int AS grp
    FROM completed
), last_dates AS (
    SELECT habit_id, MAX(local_date) AS last_date, COUNT(*)::int AS total
    FROM completed GROUP BY habit_id
)
SELECT h.id AS habit_id,
       CASE WHEN ld.last_date IS NULL OR ld.last_date < t.d - 1 THEN 0
            ELSE (SELECT COUNT(*) FROM islands i
                  WHERE i.habit_id = ld.habit_id AND i.grp = (ld.last_date - ld.total))
       END::int AS streak
FROM habits h
LEFT JOIN last_dates ld ON ld.habit_id = h.id
CROSS JOIN today t
WHERE h.user_id = $1;

-- name: GetHabitStreak :one
-- Computes the current streak for a single habit (see GetHabitStreaks).
WITH user_tz AS (
    SELECT COALESCE(s.timezone, 'UTC') AS tz FROM user_settings s WHERE s.user_id = $2
), today AS (
    SELECT (NOW() AT TIME ZONE COALESCE((SELECT tz FROM user_tz), 'UTC'))::date AS d
), completed AS (
    SELECT ci.local_date
    FROM check_ins ci
    WHERE ci.user_id = $2 AND ci.habit_id = $1 AND ci.status = 'completed'
), islands AS (
    SELECT local_date,
           local_date - (ROW_NUMBER() OVER (ORDER BY local_date))::int AS grp
    FROM completed
), last_date AS (
    SELECT MAX(local_date) AS last_date, COUNT(*)::int AS total FROM completed
)
SELECT CASE WHEN ld.last_date IS NULL OR ld.last_date < t.d - 1 THEN 0
            ELSE (SELECT COUNT(*) FROM islands i WHERE i.grp = (ld.last_date - ld.total))
       END::int AS streak
FROM last_date ld CROSS JOIN today t;

-- name: ResetTodayHabits :execrows
-- "Uncompletes" all of today's habits by deleting today's completed check-ins.
-- The streak is derived from check_ins history, so it recomputes automatically
-- once today's completed check-in is gone; no streak mutation is needed here.
-- Returns the number of completed check-ins removed.
WITH today AS (
    SELECT (now() AT TIME ZONE COALESCE(
        (SELECT s.timezone FROM user_settings s WHERE s.user_id = $1), 'UTC'))::date AS d
)
DELETE FROM check_ins ci
WHERE ci.user_id = $1
  AND ci.status = 'completed'
  AND ci.local_date = (SELECT d FROM today);

-- name: GetHabitsByIDs :many
SELECT h.id, h.user_id, h.category_id, h.name, h.description, h.created_at, h.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       EXISTS (
           SELECT 1 FROM check_ins ci
           WHERE ci.habit_id = h.id AND ci.status = 'completed'
             AND ci.local_date = (now() AT TIME ZONE COALESCE(
                 (SELECT s.timezone FROM user_settings s WHERE s.user_id = h.user_id), 'UTC'))::date
       ) AS completed
FROM habits h
LEFT JOIN categories c ON c.id = h.category_id
WHERE h.id = ANY($1::uuid[]);

-- name: ListHabitHistory :many
-- Returns completed check-ins for a user's habits within the last 28 days
-- (in the owner's timezone). Used to render the per-habit 28-day contribution
-- graph on the habit card.
WITH user_tz AS (
    SELECT COALESCE(timezone, 'UTC') AS tz
    FROM user_settings
    WHERE user_id = $1
), bounds AS (
    SELECT
        (now() AT TIME ZONE (SELECT tz FROM user_tz))::date AS today,
        (now() AT TIME ZONE (SELECT tz FROM user_tz))::date - 27 AS start_date
)
SELECT ci.habit_id, ci.local_date
FROM check_ins ci, bounds b
WHERE ci.user_id = $1
  AND ci.status = 'completed'
  AND ci.local_date >= b.start_date
  AND ci.local_date <= b.today
ORDER BY ci.local_date;

-- name: CountHabitsByUser :one
SELECT COUNT(*) FROM habits WHERE user_id = $1;

-- name: ListHabitsKeyset :many
-- Keyset pagination: pass last_created_at from the previous page (or NULL).
SELECT h.id, h.user_id, h.category_id, h.name, h.description, h.created_at, h.updated_at,
       COALESCE(c.slug, '')::varchar AS category,
       EXISTS (
           SELECT 1 FROM check_ins ci
           WHERE ci.habit_id = h.id AND ci.status = 'completed'
             AND ci.local_date = (now() AT TIME ZONE COALESCE(
                 (SELECT s.timezone FROM user_settings s WHERE s.user_id = h.user_id), 'UTC'))::date
       ) AS completed
FROM habits h
LEFT JOIN categories c ON c.id = h.category_id
WHERE h.user_id = $1
  AND ($2::timestamptz IS NULL OR h.created_at < $2)
ORDER BY h.created_at DESC
LIMIT $3;
