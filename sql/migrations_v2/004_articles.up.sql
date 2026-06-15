CREATE TABLE articles (
    id                uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    category_id       uuid REFERENCES categories(id) ON DELETE SET NULL,
    title             varchar(200) NOT NULL CHECK (length(trim(title)) > 0),
    excerpt           text,
    content           text NOT NULL,
    author            varchar(100) NOT NULL,
    read_time_minutes integer NOT NULL CHECK (read_time_minutes > 0),
    image_url         varchar(500),
    ai_metadata       jsonb NOT NULL DEFAULT '{}',
    published_at      timestamptz NOT NULL DEFAULT now(),
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),

    -- kept in sync automatically; no trigger needed
    search_vector tsvector GENERATED ALWAYS AS (
        to_tsvector('english',
            title || ' ' || coalesce(excerpt, '') || ' ' || content || ' ' || author)
    ) STORED
);

CREATE INDEX idx_articles_category_published ON articles (category_id, published_at DESC);
CREATE INDEX idx_articles_published_at ON articles (published_at DESC);
CREATE INDEX idx_articles_search ON articles USING gin (search_vector);

CREATE TRIGGER articles_set_updated_at
    BEFORE UPDATE ON articles
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
