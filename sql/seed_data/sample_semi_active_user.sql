-- ============================================================
-- Semi-active test user for the Growth platform.
--
-- Creates a user who has been using the product for ~13 days:
--   - has goals, habits, and goal<->habit links
--   - makes check-ins on ~9 of the last 13 days (semi-active)
--   - has a short multi-turn coach conversation
--   - email verified, onboarding complete, free subscription
--
-- Idempotent: safe to re-run. All child rows are scoped to this
-- one test user (looked up by email) and replaced on each run, so
-- no stale duplicates accumulate. Only this test user is touched.
--
-- Run after migrations + categories seed:
--   psql -U postgres -d growth -f seed_data/sample_semi_active_user.sql
--
-- To remove this user and ALL of their data (including ai-coach
-- conversations, which the normal account-deletion flow does NOT
-- clean up — ai-coach has no user_deleted consumer), run:
--   psql -U postgres -d growth -f seed_data/sample_semi_active_user_delete.sql
-- ============================================================

BEGIN;

-- Password hash below is bcrypt of "Password1!" with cost 10
-- (bcrypt.DefaultCost), generated via golang.org/x/crypto/bcrypt.
-- "Password1!" satisfies the registration validator (>=8 chars, upper,
-- lower, digit, special). The previous hash here was a commonly-
-- miscopied internet fixture that did NOT actually verify against
-- "password", so logins returned 401.
-- Login: semiaactive.test@example.com / Password1!
INSERT INTO users (username, email, password_hash, full_name, bio, interests,
                   email_verified, created_at, updated_at)
VALUES (
    'semiaactive_test',
    'semiaactive.test@example.com',
    '$2a$10$/3.QTcUk7ugya9UxzGJzl.GjeFEBqkZWRGn7n7I19zCZvdcxSIIEe',
    'Semi Active Tester',
    'Testing the Growth platform — semi-active user.',
    ARRAY['work', 'self-knowledge', 'calm'],
    true,
    now() - interval '13 days',
    now() - interval '13 days'
)
ON CONFLICT (email) DO UPDATE SET
    full_name       = EXCLUDED.full_name,
    bio             = EXCLUDED.bio,
    interests       = EXCLUDED.interests,
    email_verified  = EXCLUDED.email_verified,
    password_hash   = EXCLUDED.password_hash;

-- Wipe this user's existing child rows so re-runs don't duplicate.
-- (conversation_messages cascade from conversations, but we delete
--  explicitly for clarity; goal_habits cascade from goals.)
DELETE FROM conversation_messages cm
    USING conversations c, users u
    WHERE c.id = cm.conversation_id
      AND c.user_id = u.id
      AND u.email = 'semiaactive.test@example.com';
DELETE FROM conversations c
    USING users u
    WHERE c.user_id = u.id AND u.email = 'semiaactive.test@example.com';
DELETE FROM check_ins ci
    USING users u
    WHERE ci.user_id = u.id AND u.email = 'semiaactive.test@example.com';
DELETE FROM goal_habits gh
    USING goals g, users u
    WHERE gh.goal_id = g.id AND g.user_id = u.id
      AND u.email = 'semiaactive.test@example.com';
DELETE FROM goals g
    USING users u
    WHERE g.user_id = u.id AND u.email = 'semiaactive.test@example.com';
DELETE FROM habits h
    USING users u
    WHERE h.user_id = u.id AND u.email = 'semiaactive.test@example.com';
DELETE FROM subscriptions s
    USING users u
    WHERE s.user_id = u.id AND u.email = 'semiaactive.test@example.com';
DELETE FROM user_preferences up
    USING users u
    WHERE up.user_id = u.id AND u.email = 'semiaactive.test@example.com';
DELETE FROM user_profiles up
    USING users u
    WHERE up.id = u.id AND u.email = 'semiaactive.test@example.com';

-- ============================================================
-- Read model + preferences + subscription
-- ============================================================
INSERT INTO user_profiles (id, username, full_name, bio, interests, created_at, updated_at)
SELECT u.id, 'semiaactive_test', 'Semi Active Tester',
       'Testing the Growth platform — semi-active user.',
       ARRAY['work', 'self-knowledge', 'calm'],
       now() - interval '13 days', now() - interval '13 days'
FROM users u
WHERE u.email = 'semiaactive.test@example.com';

INSERT INTO user_preferences (user_id, onboarding_completed, timezone, created_at, updated_at)
SELECT u.id, true, 'UTC',
       now() - interval '13 days', now() - interval '13 days'
FROM users u
WHERE u.email = 'semiaactive.test@example.com';

INSERT INTO subscriptions (user_id, plan_id, status, current_period_start, current_period_end)
SELECT u.id, p.id, 'free',
       now() - interval '13 days', now() + interval '1 year'
FROM users u, plans p
WHERE u.email = 'semiaactive.test@example.com' AND p.code = 'free';

-- ============================================================
-- Goals (3 active, backdated ~13 days)
-- ============================================================
INSERT INTO goals (user_id, category_id, title, description, status, progress, due_date, created_at, updated_at)
SELECT u.id, c.id, 'Build a consistent morning routine',
        'Meditate and read every morning for 30 days',
        'active', 35, now() + interval '17 days',
        now() - interval '13 days', now() - interval '13 days'
FROM users u, categories c
WHERE u.email = 'semiaactive.test@example.com' AND c.slug = 'calm'
UNION ALL
SELECT u.id, c.id, 'Read 12 books this year',
        'One book per month — currently on track',
        'active', 25, now() + interval '9 months',
        now() - interval '13 days', now() - interval '13 days'
FROM users u, categories c
WHERE u.email = 'semiaactive.test@example.com' AND c.slug = 'leisure'
UNION ALL
SELECT u.id, c.id, 'Ship a side project MVP',
        'Four-week sprint to a usable MVP',
        'active', 15, now() + interval '15 days',
        now() - interval '13 days', now() - interval '13 days'
FROM users u, categories c
WHERE u.email = 'semiaactive.test@example.com' AND c.slug = 'work';

-- ============================================================
-- Habits (4, backdated ~13 days)
-- ============================================================
INSERT INTO habits (user_id, category_id, name, description, status, reminder_time, created_at, updated_at)
SELECT u.id, c.id, 'Morning meditation',
        '3–10 minutes of mindfulness after coffee',
        'active', '07:30'::time,
        now() - interval '13 days', now() - interval '13 days'
FROM users u, categories c
WHERE u.email = 'semiaactive.test@example.com' AND c.slug = 'calm'
UNION ALL
SELECT u.id, c.id, 'Read 10 pages',
        'Non-fiction personal growth reading',
        'active', '21:00'::time,
        now() - interval '13 days', now() - interval '13 days'
FROM users u, categories c
WHERE u.email = 'semiaactive.test@example.com' AND c.slug = 'work'
UNION ALL
SELECT u.id, c.id, 'Evening walk',
        '20-minute walk to unwind',
        'active', '18:30'::time,
        now() - interval '13 days', now() - interval '13 days'
FROM users u, categories c
WHERE u.email = 'semiaactive.test@example.com' AND c.slug = 'leisure'
UNION ALL
SELECT u.id, c.id, 'Drink 2L water',
        'Stay hydrated through the day',
        'active', NULL,
        now() - interval '13 days', now() - interval '13 days'
FROM users u, categories c
WHERE u.email = 'semiaactive.test@example.com' AND c.slug = 'self-knowledge';

-- ============================================================
-- Link goals to habits
-- ============================================================
INSERT INTO goal_habits (goal_id, habit_id)
SELECT g.id, h.id
FROM goals g, habits h, users u
WHERE g.user_id = u.id AND h.user_id = u.id
  AND u.email = 'semiaactive.test@example.com'
  AND g.title = 'Build a consistent morning routine'
  AND h.name  = 'Morning meditation'
UNION ALL
SELECT g.id, h.id
FROM goals g, habits h, users u
WHERE g.user_id = u.id AND h.user_id = u.id
  AND u.email = 'semiaactive.test@example.com'
  AND g.title = 'Read 12 books this year'
  AND h.name  = 'Read 10 pages'
ON CONFLICT DO NOTHING;

-- ============================================================
-- Check-ins — semi-active: 9 of the last 13 days
-- (local_date uses the calendar day; created_at is backdated
--  to match so the activity timeline looks realistic)
-- ============================================================
INSERT INTO check_ins (user_id, habit_id, local_date, status, mood, energy, note, created_at)
SELECT u.id, h.id, (now() - interval '13 days')::date, 'completed', 'okay',  'medium', NULL, now() - interval '13 days'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Morning meditation'
UNION ALL
SELECT u.id, h.id, (now() - interval '12 days')::date, 'completed', 'great', 'high',   'Felt calm afterward', now() - interval '12 days'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Morning meditation'
UNION ALL
SELECT u.id, h.id, (now() - interval '12 days')::date, 'completed', 'great', 'high',   NULL, now() - interval '12 days'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Read 10 pages'
UNION ALL
SELECT u.id, h.id, (now() - interval '10 days')::date, 'completed', 'okay',  'medium', NULL, now() - interval '10 days'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Morning meditation'
UNION ALL
SELECT u.id, h.id, (now() - interval '10 days')::date, 'completed', 'okay',  'low',    NULL, now() - interval '10 days'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Evening walk'
UNION ALL
SELECT u.id, h.id, (now() - interval '9 days')::date,  'completed', 'low',   'low',    'Rushed morning', now() - interval '9 days'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Morning meditation'
UNION ALL
SELECT u.id, h.id, (now() - interval '7 days')::date,  'completed', 'okay',  'medium', NULL, now() - interval '7 days'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Read 10 pages'
UNION ALL
SELECT u.id, h.id, (now() - interval '7 days')::date,  'completed', 'okay',  'medium', NULL, now() - interval '7 days'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Evening walk'
UNION ALL
SELECT u.id, h.id, (now() - interval '5 days')::date,  'completed', 'great', 'high',   NULL, now() - interval '5 days'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Morning meditation'
UNION ALL
SELECT u.id, h.id, (now() - interval '5 days')::date,  'completed', 'okay',  'medium', NULL, now() - interval '5 days'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Drink 2L water'
UNION ALL
SELECT u.id, h.id, (now() - interval '4 days')::date,  'completed', 'okay',  'medium', NULL, now() - interval '4 days'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Morning meditation'
UNION ALL
SELECT u.id, h.id, (now() - interval '4 days')::date,  'missed',    'low',   'low',    'lack_of_time', now() - interval '4 days'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Read 10 pages'
UNION ALL
SELECT u.id, h.id, (now() - interval '2 days')::date,  'completed', 'okay',  'medium', NULL, now() - interval '2 days'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Evening walk'
UNION ALL
SELECT u.id, h.id, (now() - interval '1 day')::date,   'completed', 'great', 'high',   'Great session', now() - interval '1 day'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Morning meditation'
UNION ALL
SELECT u.id, h.id, (now() - interval '1 day')::date,   'completed', 'okay',  'medium', NULL, now() - interval '1 day'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Read 10 pages'
UNION ALL
SELECT u.id, h.id, (now() - interval '1 day')::date,   'completed', 'okay',  'high',   NULL, now() - interval '1 day'
FROM users u, habits h
WHERE u.email = 'semiaactive.test@example.com' AND h.user_id = u.id AND h.name = 'Drink 2L water'
ON CONFLICT (habit_id, local_date) DO NOTHING;

-- ============================================================
-- Coach conversation (1) + messages (6, alternating user/assistant)
-- Spread across the first two days of usage.
-- ============================================================
WITH conv AS (
    INSERT INTO conversations (user_id, title, type, last_message, created_at, updated_at)
    SELECT u.id, 'Getting started with meditation', 'coach',
           'Perfect. I''ve noted morning meditation as your focus. Check in tomorrow and let me know how the 3-minute session went!',
           now() - interval '12 days', now() - interval '11 days'
    FROM users u
    WHERE u.email = 'semiaactive.test@example.com'
    RETURNING id AS conv_id
)
INSERT INTO conversation_messages (conversation_id, role, content, created_at)
SELECT conv_id, role, content, created_at
FROM conv
CROSS JOIN (VALUES
    ('user',      'I''m struggling to keep up with my meditation habit, any tips?',                    now() - interval '12 days'),
    ('assistant', 'It''s completely normal to find consistency hard at first. Let''s start small — could you commit to just 3 minutes of meditation for the next three days?', now() - interval '12 days'),
    ('user',      'Yes, 3 minutes feels doable. What time of day works best?',                        now() - interval '12 days'),
    ('assistant', 'Morning tends to work well before the day gets busy. Try linking it to an existing habit like your morning coffee. Shall we set a reminder?', now() - interval '12 days'),
    ('user',      'Morning with coffee sounds great. Let''s do it.',                                   now() - interval '11 days'),
    ('assistant', 'Perfect. I''ve noted morning meditation as your focus. Check in tomorrow and let me know how the 3-minute session went!', now() - interval '11 days')
) AS t(role, content, created_at);

COMMIT;

-- ============================================================
-- Done. Test user:
--   username: semiaactive_test
--   email:    semiaactive.test@example.com
--   password: Password1!
-- ============================================================
