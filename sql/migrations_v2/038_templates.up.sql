-- Habit & goal templates — admin-managed suggestion library shown on the
-- explore page. Mirrors the habits/goals shape but without user ownership.
-- category_id references the shared categories table.

CREATE TABLE habit_templates (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    name        varchar(100) NOT NULL CHECK (length(trim(name)) > 0),
    description text,
    category_id uuid REFERENCES categories(id) ON DELETE SET NULL,
    sort_order  int NOT NULL DEFAULT 0,
    is_active   boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_habit_templates_active_sort ON habit_templates (is_active, sort_order);

CREATE TRIGGER habit_templates_set_updated_at
    BEFORE UPDATE ON habit_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

CREATE TABLE goal_templates (
    id          uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    title       varchar(200) NOT NULL CHECK (length(trim(title)) > 0),
    description text,
    category_id uuid REFERENCES categories(id) ON DELETE SET NULL,
    sort_order  int NOT NULL DEFAULT 0,
    is_active   boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_goal_templates_active_sort ON goal_templates (is_active, sort_order);

CREATE TRIGGER goal_templates_set_updated_at
    BEFORE UPDATE ON goal_templates
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
