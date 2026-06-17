CREATE TABLE tags (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    name       varchar(50) NOT NULL UNIQUE,
    slug       varchar(50) NOT NULL UNIQUE,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE article_tags (
    article_id uuid NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    tag_id     uuid NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (article_id, tag_id)
);

CREATE INDEX idx_article_tags_tag ON article_tags(tag_id);