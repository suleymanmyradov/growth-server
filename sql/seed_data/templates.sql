-- Seed habit & goal templates from the previously hardcoded frontend values.
-- Category IDs are resolved by slug at insert time.

INSERT INTO habit_templates (name, description, category_id, sort_order)
SELECT
    t.name, t.description, c.id, t.sort_order
FROM (VALUES
    ('Morning Walk', '15-minute walk to start the day fresh', 'leisure', 1),
    ('Read 10 pages', 'Non-fiction personal growth', 'work', 2),
    ('Meditate', '5–10 minutes of mindfulness', 'calm', 3)
) AS t(name, description, slug, sort_order)
JOIN categories c ON c.slug = t.slug
ON CONFLICT DO NOTHING;

INSERT INTO goal_templates (title, description, category_id, sort_order)
SELECT
    g.title, g.description, c.id, g.sort_order
FROM (VALUES
    ('Ship a side project', 'MVP within 4 weeks', 'work', 1),
    ('Run 5K', 'Train 3x weekly for 6 weeks', 'leisure', 2),
    ('30-day meditation', 'Daily 10 minutes', 'calm', 3)
) AS g(title, description, slug, sort_order)
JOIN categories c ON c.slug = g.slug
ON CONFLICT DO NOTHING;
