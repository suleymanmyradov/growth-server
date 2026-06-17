-- ============================================================
-- Sample seed data for the Growth project.
-- Insert order respects FK dependencies.
-- Uses subqueries for FK lookups so UUIDs are never hardcoded.
-- Run after all migrations have been applied.
-- ============================================================
-- Usage: psql -U postgres -d growth -f seed_data/sample.sql
-- ============================================================

BEGIN;

-- ============================================================
-- 1. Categories (shared by articles, goals, habits)
--    Slugs match frontend constants: GOAL_CATEGORIES / HABIT_CATEGORIES
-- ============================================================
INSERT INTO categories (name, slug, sort_order) VALUES
    ('Calm', 'calm', 1),
    ('Leisure',        'leisure',       2),
    ('Relationships',   'relationships',  3),
    ('Self-Knowledge',  'self-knowledge', 4),
    ('Sociability',     'sociability',    5),
    ('Work',           'work',           6)
ON CONFLICT (slug) DO NOTHING;

-- ============================================================
-- 2. Demo user
--    Password hash below is bcrypt of "password" with cost 10.
--    Generate a new one with:
--      htpasswd -bnBC 10 "" password | tr -d ':\n'
--    or in Go:
--      bcrypt.GenerateFromPassword([]byte("password"), 10)
-- ============================================================
WITH u AS (
  INSERT INTO users (username, email, password_hash, full_name, bio, interests)
  VALUES (
    'demo',
    'demo@example.com',
    '$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy',
    'Demo User',
    'A demo user exploring the Growth platform.',
    ARRAY['work', 'self-knowledge', 'relationships']
  )
  ON CONFLICT (email) DO UPDATE SET
    full_name = EXCLUDED.full_name
  RETURNING id
)
-- ============================================================
-- 3. User settings (defaults are fine for a demo)
-- ============================================================
INSERT INTO user_settings (user_id, onboarding_completed)
SELECT id, true FROM u
ON CONFLICT (user_id) DO NOTHING;

-- ============================================================
-- 3b. Free subscription for the demo user
--      (the app also creates this on demand via CreateDefaultFreeSubscription)
-- ============================================================
INSERT INTO subscriptions (user_id, plan_id, status)
SELECT u.id, p.id, 'free'
FROM users u, plans p
WHERE u.email = 'demo@example.com' AND p.code = 'free'
ON CONFLICT (user_id) DO NOTHING;

-- ============================================================
-- 4. Articles — 2 per category = 12 articles
-- ============================================================
WITH
  calm_cat       AS (SELECT id FROM categories WHERE slug = 'calm'),
  leisure_cat    AS (SELECT id FROM categories WHERE slug = 'leisure'),
  relationships_cat AS (SELECT id FROM categories WHERE slug = 'relationships'),
  self_know_cat  AS (SELECT id FROM categories WHERE slug = 'self-knowledge'),
  sociability_cat AS (SELECT id FROM categories WHERE slug = 'sociability'),
  work_cat       AS (SELECT id FROM categories WHERE slug = 'work')
INSERT INTO articles (category_id, title, excerpt, content, author, read_time_minutes, image_url, published_at)
SELECT cc.id, 'Digital Detox: Reclaim Your Attention',
       'Why stepping away from screens is essential for mental clarity.',
       'The average person spends over 6 hours per day on digital devices. This constant connectivity fragments our attention and elevates stress levels. A digital detox — even for a few hours — can reset your focus, improve sleep, and deepen real-world connections. Practical strategies for reducing screen time without missing what matters.',
       'Growth Editorial', 7, NULL, now() - interval '2 days'
FROM calm_cat cc
UNION ALL
SELECT cc.id, 'The Art of Doing Nothing',
       'Why stillness and unstructured time are essential for well-being.',
       'In a culture obsessed with productivity, doing nothing has become a radical act. Yet moments of unstructured stillness are essential for creativity, emotional processing, and mental recovery. This article explores the science of rest and how to incorporate guilt-free pauses into your daily life.',
       'Growth Editorial', 5, NULL, now() - interval '5 days'
FROM calm_cat cc
UNION ALL
SELECT lc.id, 'Why Walking Is the Ultimate Exercise',
       'Low impact, high reward — the science behind daily walks.',
       'Walking is often underrated as a form of exercise, but the evidence is overwhelming. Regular walking reduces the risk of chronic diseases, improves mental health, strengthens bones, and can add years to your life. This article explores why a daily 30-minute walk might be the single best investment in your health.',
       'Growth Editorial', 6, NULL, now() - interval '3 days'
FROM leisure_cat lc
UNION ALL
SELECT lc.id, 'The Joy of Reading for Pleasure',
       'How recreational reading expands empathy and reduces stress.',
       'Reading for pleasure is more than just entertainment. Studies show that recreational reading improves empathy, reduces stress by up to 68%, and keeps your brain sharp as you age. Rediscover the joy of getting lost in a good book with practical tips to make reading a regular part of your leisure time.',
       'Growth Editorial', 5, NULL, now() - interval '7 days'
FROM leisure_cat lc
UNION ALL
SELECT rc.id, 'The Art of Active Listening',
       'How to make people feel truly heard.',
       'Most people listen to reply, not to understand. Active listening is a skill that transforms relationships — it builds trust, deepens connection, and resolves conflicts before they escalate. This article breaks down the techniques used by therapists and negotiators that anyone can practice in everyday conversations.',
       'Growth Editorial', 6, NULL, now() - interval '4 days'
FROM relationships_cat rc
UNION ALL
SELECT rc.id, 'Setting Healthy Boundaries',
       'Why boundaries are the foundation of strong relationships.',
       'Boundaries are not walls — they are the guidelines that protect your time, energy, and emotional well-being. Without them, resentment builds and relationships suffer. Learn how to identify your limits, communicate them clearly, and maintain them with compassion.',
       'Growth Editorial', 7, NULL, now() - interval '9 days'
FROM relationships_cat rc
UNION ALL
SELECT skc.id, 'Getting Started with Meditation',
       'A beginner-friendly guide to mindfulness meditation.',
       'Meditation has been practiced for thousands of years, and modern science confirms its benefits: reduced stress, improved focus, lower anxiety, and greater emotional resilience. If you have never meditated before, this step-by-step guide will walk you through your first sessions and help you build a sustainable practice.',
       'Growth Editorial', 5, NULL, now() - interval '4 days'
FROM self_know_cat skc
UNION ALL
SELECT skc.id, 'The Art of Journaling',
       'How reflective writing can clarify your thoughts and reduce stress.',
       'Journaling is one of the simplest and most effective mental health practices. By putting your thoughts on paper, you gain perspective, process emotions, and identify patterns in your behavior. This article explores different journaling methods — from gratitude journals to stream-of-consciousness writing — and how to make the habit stick.',
       'Growth Editorial', 6, NULL, now() - interval '8 days'
FROM self_know_cat skc
UNION ALL
SELECT sc.id, 'How to Start a Conversation with Anyone',
       'Simple techniques to overcome awkwardness and connect instantly.',
       'Social situations can be intimidating, but conversation is a skill anyone can learn. From open-ended questions to mirroring body language, small adjustments can make you more approachable and engaging. This guide covers practical techniques to start conversations, keep them flowing, and leave a positive impression.',
       'Growth Editorial', 6, NULL, now() - interval '6 days'
FROM sociability_cat sc
UNION ALL
SELECT sc.id, 'The Power of Community',
       'Why belonging is a fundamental human need.',
       'Humans are social creatures. Studies show that people with strong community ties live longer, report higher happiness, and recover faster from illness. Whether it is a book club, a sports team, or a neighborhood group, finding your people is one of the most impactful things you can do for your well-being.',
       'Growth Editorial', 7, NULL, now() - interval '11 days'
FROM sociability_cat sc
UNION ALL
SELECT wc.id, 'Master Your Morning Routine',
       'How to structure the first hour of your day for peak productivity.',
       'Starting your day with intention sets the tone for everything that follows. Research shows that successful people follow consistent morning routines that include elements like exercise, planning, and focused work before noon. In this article, we explore the key components of an effective morning routine and how to customize it to your personal goals and energy levels.',
       'Growth Editorial', 5, NULL, now() - interval '2 days'
FROM work_cat wc
UNION ALL
SELECT wc.id, 'The Power of Deep Work',
       'Why focused, uninterrupted work produces your best results.',
       'Deep work — the ability to focus without distraction on a cognitively demanding task — is becoming increasingly rare and valuable. Cal Newport popularized this concept, showing that deep work produces more output in one hour than shallow work produces in an entire day. Learn practical strategies to cultivate this superpower in a world full of distractions.',
       'Growth Editorial', 7, NULL, now() - interval '5 days'
FROM work_cat wc;

-- ============================================================
-- 5. Demo goals
-- ============================================================
WITH demo_user AS (
  SELECT id FROM users WHERE email = 'demo@example.com'
), calm_cat AS (
  SELECT id FROM categories WHERE slug = 'calm'
), leisure_cat AS (
  SELECT id FROM categories WHERE slug = 'leisure'
), self_know_cat AS (
  SELECT id FROM categories WHERE slug = 'self-knowledge'
), work_cat AS (
  SELECT id FROM categories WHERE slug = 'work'
)
INSERT INTO goals (user_id, category_id, title, description, status, progress, due_date)
SELECT du.id, cc.id, '30-day meditation challenge',
       'Daily 10 minutes of mindfulness meditation',
       'active', 0, now() + interval '30 days'
FROM demo_user du, calm_cat cc
UNION ALL
SELECT du.id, lc.id, 'Read 12 books this year',
       'One book per month for personal enjoyment and growth',
       'active', 0, now() + interval '12 months'
FROM demo_user du, leisure_cat lc
UNION ALL
SELECT du.id, skc.id, 'Complete a personality journal',
       'Daily journaling for 90 days to deepen self-understanding',
       'active', 0, now() + interval '90 days'
FROM demo_user du, self_know_cat skc
UNION ALL
SELECT du.id, wc.id, 'Ship a side project',
       'MVP within 4 weeks',
       'active', 0, now() + interval '4 weeks'
FROM demo_user du, work_cat wc;

-- ============================================================
-- 6. Demo habits
-- ============================================================
WITH demo_user AS (
  SELECT id FROM users WHERE email = 'demo@example.com'
), calm_cat AS (
  SELECT id FROM categories WHERE slug = 'calm'
), leisure_cat AS (
  SELECT id FROM categories WHERE slug = 'leisure'
), relationships_cat AS (
  SELECT id FROM categories WHERE slug = 'relationships'
), self_know_cat AS (
  SELECT id FROM categories WHERE slug = 'self-knowledge'
), sociability_cat AS (
  SELECT id FROM categories WHERE slug = 'sociability'
), work_cat AS (
  SELECT id FROM categories WHERE slug = 'work'
)
INSERT INTO habits (user_id, category_id, name, description)
SELECT du.id, cc.id, 'Evening wind-down',
       '30 minutes of screen-free relaxation before bed'
FROM demo_user du, calm_cat cc
UNION ALL
SELECT du.id, lc.id, 'Morning walk',
       '15-minute walk to start the day fresh'
FROM demo_user du, leisure_cat lc
UNION ALL
SELECT du.id, rc.id, 'Check in with a friend',
       'Send one meaningful message to a friend each day'
FROM demo_user du, relationships_cat rc
UNION ALL
SELECT du.id, skc.id, 'Meditate',
       '5-10 minutes of mindfulness meditation'
FROM demo_user du, self_know_cat skc
UNION ALL
SELECT du.id, sc.id, 'One social outing per week',
       'Attend a group event or meet someone new weekly'
FROM demo_user du, sociability_cat sc
UNION ALL
SELECT du.id, wc.id, 'Read 10 pages',
       'Non-fiction personal growth reading'
FROM demo_user du, work_cat wc;

-- ============================================================
-- 7. Link goals to habits
-- ============================================================
WITH demo_user AS (
  SELECT id FROM users WHERE email = 'demo@example.com'
), meditate_goal AS (
  SELECT g.id FROM goals g, demo_user du
  WHERE g.user_id = du.id AND g.title = '30-day meditation challenge'
), meditate_habit AS (
  SELECT h.id FROM habits h, demo_user du
  WHERE h.user_id = du.id AND h.name = 'Meditate'
), read_goal AS (
  SELECT g.id FROM goals g, demo_user du
  WHERE g.user_id = du.id AND g.title = 'Read 12 books this year'
), read_habit AS (
  SELECT h.id FROM habits h, demo_user du
  WHERE h.user_id = du.id AND h.name = 'Read 10 pages'
)
INSERT INTO goal_habits (goal_id, habit_id)
SELECT mg.id, mh.id FROM meditate_goal mg, meditate_habit mh
UNION ALL
SELECT rg.id, rh.id FROM read_goal rg, read_habit rh
ON CONFLICT DO NOTHING;

COMMIT;
