-- Reverse migration 032: drop the user_memory outbox triggers and restore the
-- original entity_type CHECK (article/goal/habit only). The shared
-- enqueue_search_event function is owned by migration 024 and is NOT dropped
-- here.

DROP TRIGGER IF EXISTS weekly_reviews_search_sync_delete ON weekly_reviews;
DROP TRIGGER IF EXISTS weekly_reviews_search_sync_upsert ON weekly_reviews;
DROP TRIGGER IF EXISTS conversation_messages_search_sync ON conversation_messages;
DROP TRIGGER IF EXISTS check_ins_search_sync_delete ON check_ins;
DROP TRIGGER IF EXISTS check_ins_search_sync_insert ON check_ins;

-- Restore the original CHECK from migration 024.
ALTER TABLE search_outbox DROP CONSTRAINT IF EXISTS search_outbox_entity_type_check;
ALTER TABLE search_outbox ADD CONSTRAINT search_outbox_entity_type_check
    CHECK (entity_type IN ('article', 'goal', 'habit'));
