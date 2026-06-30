-- Workstream 2: long-term retrieval memory for the AI coach.
-- Widens the search_outbox entity_type CHECK to also accept the free-text
-- sources that feed the private `user_memory` Meili index, and adds per-row
-- triggers reusing the existing enqueue_search_event(TG_ARGV[0]) function.
--
-- The public catalog (article/goal/habit) rows keep flowing to the existing
-- index; the search-sync worker routes the new entity types to user_memory.
-- See services/microservices/search-sync/internal/syncer/syncer.go.

-- Widen the entity_type domain. The original constraint was created unnamed in
-- migration 024, so Postgres assigned the default name
-- `search_outbox_entity_type_check`.
ALTER TABLE search_outbox DROP CONSTRAINT IF EXISTS search_outbox_entity_type_check;
ALTER TABLE search_outbox ADD CONSTRAINT search_outbox_entity_type_check
    CHECK (entity_type IN (
        'article', 'goal', 'habit',
        'check_in', 'conversation_message', 'weekly_review'
    ));

-- check_ins are immutable (no UPDATE). Only index rows that actually carry a
-- free-text note. A single WHEN clause cannot cover both INSERT (NEW-bound) and
-- DELETE (OLD-bound), so the upsert and delete paths get separate triggers.
DROP TRIGGER IF EXISTS check_ins_search_sync_insert ON check_ins;
CREATE TRIGGER check_ins_search_sync_insert
    AFTER INSERT ON check_ins
    FOR EACH ROW
    WHEN (NEW.note IS NOT NULL AND NEW.note <> '')
    EXECUTE FUNCTION enqueue_search_event('check_in');

DROP TRIGGER IF EXISTS check_ins_search_sync_delete ON check_ins;
CREATE TRIGGER check_ins_search_sync_delete
    AFTER DELETE ON check_ins
    FOR EACH ROW
    EXECUTE FUNCTION enqueue_search_event('check_in');

-- conversation_messages.content is NOT NULL, so every row is indexable.
-- Cascade deletes (conversations -> conversation_messages) fire per-row DELETE
-- triggers, so index deletes are enqueued automatically when a conversation is
-- removed.
DROP TRIGGER IF EXISTS conversation_messages_search_sync ON conversation_messages;
CREATE TRIGGER conversation_messages_search_sync
    AFTER INSERT OR DELETE ON conversation_messages
    FOR EACH ROW
    EXECUTE FUNCTION enqueue_search_event('conversation_message');

-- weekly_reviews.ai_summary is filled in after creation (via UPDATE), so the
-- trigger fires on INSERT or UPDATE OF ai_summary and only when the summary is
-- non-empty. A DELETE trigger is included so user/account deletion (which
-- cascades via FK) cleans the index.
DROP TRIGGER IF EXISTS weekly_reviews_search_sync_upsert ON weekly_reviews;
CREATE TRIGGER weekly_reviews_search_sync_upsert
    AFTER INSERT OR UPDATE OF ai_summary ON weekly_reviews
    FOR EACH ROW
    WHEN (NEW.ai_summary IS NOT NULL AND NEW.ai_summary <> '')
    EXECUTE FUNCTION enqueue_search_event('weekly_review');

DROP TRIGGER IF EXISTS weekly_reviews_search_sync_delete ON weekly_reviews;
CREATE TRIGGER weekly_reviews_search_sync_delete
    AFTER DELETE ON weekly_reviews
    FOR EACH ROW
    EXECUTE FUNCTION enqueue_search_event('weekly_review');
