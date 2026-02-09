-- Drop shared functions
-- WARNING: Only run this when ALL dependent tables have been dropped

-- Check if any triggers depend on this function before dropping
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_trigger t
        JOIN pg_proc p ON t.tgfoid = p.oid
        WHERE p.proname = 'update_updated_at_column'
    ) THEN
        DROP FUNCTION IF EXISTS update_updated_at_column();
    ELSE
        RAISE NOTICE 'update_updated_at_column() not dropped: triggers still depend on it';
    END IF;
END $$;

-- Drop compatibility uuid_generate_v7 if present
DROP FUNCTION IF EXISTS uuid_generate_v7();

-- Drop UUID extension if no longer needed
DROP EXTENSION IF EXISTS pgcrypto;
