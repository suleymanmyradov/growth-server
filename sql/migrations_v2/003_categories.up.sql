-- Single global category list shared by articles, goals, and habits.
CREATE TABLE categories (
    id         uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    name       varchar(50) NOT NULL,
    slug       varchar(50) NOT NULL UNIQUE,
    sort_order integer NOT NULL DEFAULT 0,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TRIGGER categories_set_updated_at
    BEFORE UPDATE ON categories
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
