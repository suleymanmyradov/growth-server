CREATE TABLE article_shares (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v7(),
    article_id UUID NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    platform VARCHAR(50) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for efficient lookups
CREATE INDEX idx_article_shares_article_id ON article_shares(article_id);
CREATE INDEX idx_article_shares_user_id ON article_shares(user_id);
