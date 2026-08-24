-- Curated long-term memory: the coach's durable facts about a user.
--
-- Why this exists alongside the user_memory search index: retrieval over raw
-- conversation messages cannot distinguish a durable commitment ("I train
-- Tuesdays and Thursdays") from a passing mood ("I feel awful today"), and it
-- has no way to express that one statement replaces an earlier one. Both are
-- write-time curation problems, not search-relevance problems -- no amount of
-- ranking tuning fixes them.
--
-- Division of labour:
--   user_facts        small, curated, injected into the prompt by default
--   user_memory index large, hybrid-searched, queried on demand via a tool
--
-- Supersession rather than mutation: a correction inserts a new row and points
-- the old one at it. The history stays auditable, which matters because these
-- rows are model-authored and a user asking "why do you think that?" deserves
-- an answer.

CREATE TABLE user_facts (
    id           uuid PRIMARY KEY DEFAULT uuid_generate_v7(),
    user_id      uuid NOT NULL,

    -- The fact as a short, self-contained statement in the user's own framing.
    fact         text NOT NULL CHECK (length(trim(fact)) > 0),

    -- Closed vocabulary, text + CHECK per the migrations_v2 design rules.
    --   commitment: something the user said they would do
    --   preference: how they want to be coached
    --   constraint: a durable limit (schedule, health, environment)
    --   context:    stable background (role, location, family situation)
    category     text NOT NULL
        CHECK (category IN ('commitment', 'preference', 'constraint', 'context')),

    -- Extraction confidence. Low-confidence guesses must never be presented as
    -- things the coach knows, so retrieval filters on this.
    confidence   real NOT NULL DEFAULT 0
        CHECK (confidence >= 0 AND confidence <= 1),

    -- Provenance: which turn produced this. Nullable because a user editing
    -- their own memory has no source message. ON DELETE SET NULL keeps the fact
    -- when its originating message is deleted -- the fact may still be true.
    source_message_id uuid REFERENCES conversation_messages(id) ON DELETE SET NULL,

    -- Set when the user (not the model) authored or corrected this fact. A
    -- user-authored fact outranks a model-extracted one.
    user_authored bool NOT NULL DEFAULT false,

    -- Supersession chain. superseded_by IS NULL means "currently believed".
    superseded_by uuid REFERENCES user_facts(id) ON DELETE SET NULL,
    superseded_at timestamptz,

    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),

    -- superseded_by and superseded_at travel together or not at all.
    CONSTRAINT user_facts_supersession_complete CHECK (
        (superseded_by IS NULL AND superseded_at IS NULL) OR
        (superseded_by IS NOT NULL AND superseded_at IS NOT NULL)
    ),
    -- A fact cannot supersede itself.
    CONSTRAINT user_facts_no_self_supersede CHECK (superseded_by IS DISTINCT FROM id)
);

CREATE TRIGGER user_facts_set_updated_at
    BEFORE UPDATE ON user_facts
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();

-- The hot path: current facts for one user, best first. Partial index because
-- superseded rows are only read when auditing history.
CREATE INDEX idx_user_facts_current
    ON user_facts (user_id, category, confidence DESC)
    WHERE superseded_by IS NULL;

-- Walking a supersession chain backwards.
CREATE INDEX idx_user_facts_superseded_by
    ON user_facts (superseded_by)
    WHERE superseded_by IS NOT NULL;

-- One live copy of the same statement per user. Prevents the extractor from
-- re-adding a fact it already recorded on every subsequent turn.
CREATE UNIQUE INDEX idx_user_facts_unique_live
    ON user_facts (user_id, lower(trim(fact)))
    WHERE superseded_by IS NULL;
