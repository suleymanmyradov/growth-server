-- Draft/publish workflow: add a status column to articles.
-- Existing rows default to 'published' so the public site is unaffected.
ALTER TABLE articles
    ADD COLUMN status varchar(20) NOT NULL DEFAULT 'published'
        CHECK (status IN ('draft', 'published'));

-- Partial index to find drafts quickly; published rows are the common case.
CREATE INDEX idx_articles_status ON articles (status);

-- Keep published articles ordered by date for the public list queries.
CREATE INDEX idx_articles_published_status ON articles (status, published_at DESC);
