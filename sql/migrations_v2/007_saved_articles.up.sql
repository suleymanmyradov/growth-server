CREATE TABLE saved_articles (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    article_id uuid NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at timestamptz NOT NULL DEFAULT now(),

    UNIQUE (article_id, user_id)
);

CREATE INDEX idx_saved_articles_user ON saved_articles (user_id, created_at DESC);
