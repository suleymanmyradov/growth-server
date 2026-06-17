INSERT INTO categories (name, slug, sort_order) VALUES
    ('Calm', 'calm', 1),
    ('Leisure',        'leisure',       2),
    ('Relationships',   'relationships',  3),
    ('Self-Knowledge',  'self-knowledge', 4),
    ('Sociability',     'sociability',    5),
    ('Work',           'work',           6)
ON CONFLICT (slug) DO NOTHING;
