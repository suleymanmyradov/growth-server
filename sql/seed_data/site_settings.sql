-- Seed default explore-page settings so the client frontend has values
-- before an admin ever touches the settings page.
INSERT INTO site_settings (key, value) VALUES
    (
        'explore.header',
        '{"title":"Explore","subtitle":"Discover content to inspire your growth journey."}'::jsonb
    ),
    (
        'explore.tabs',
        '["articles","habits","goals","community"]'::jsonb
    ),
    (
        'community.card',
        '{"title":"Connect with the Community","description":"Join others on their growth journey. Share insights, get support, and stay motivated.","discordUrl":"https://discord.com/invite/your-server","xUrl":"https://x.com/your-handle"}'::jsonb
    )
ON CONFLICT (key) DO NOTHING;
