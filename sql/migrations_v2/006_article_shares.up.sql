CREATE TABLE article_shares (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    article_id uuid NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform   varchar(50) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),

    UNIQUE (article_id, user_id, platform)
);

CREATE INDEX idx_article_shares_user ON article_shares (user_id, created_at DESC);
