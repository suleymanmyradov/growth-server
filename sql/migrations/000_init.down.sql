DO $$
DECLARE
    r RECORD;
BEGIN
    -- Drop all tables with cascade (except schema_migrations)
    FOR r IN (SELECT tablename FROM pg_tables WHERE schemaname='public' AND tablename != 'schema_migrations' ORDER BY tablename) LOOP
        EXECUTE format('DROP TABLE IF EXISTS %I.%I CASCADE', 'public', r.tablename);
    END LOOP;

    -- Drop all custom functions (not from extensions)
    FOR r IN (
        SELECT p.proname, pg_get_function_identity_arguments(p.oid) as args
        FROM pg_proc p
        JOIN pg_namespace n ON p.pronamespace = n.oid
        LEFT JOIN pg_depend d ON d.objid = p.oid AND d.deptype = 'e'
        WHERE n.nspname = 'public' AND d.objid IS NULL
        ORDER BY p.proname
    ) LOOP
        EXECUTE format('DROP FUNCTION IF EXISTS %I(%s) CASCADE', r.proname, r.args);
    END LOOP;

    -- Drop all custom types (not from extensions)
    FOR r IN (
        SELECT t.typname
        FROM pg_type t
        JOIN pg_namespace n ON t.typnamespace = n.oid
        LEFT JOIN pg_depend d ON d.objid = t.oid AND d.deptype = 'e'
        WHERE n.nspname = 'public' AND t.typtype = 'e' AND d.objid IS NULL
        ORDER BY t.typname
    ) LOOP
        EXECUTE format('DROP TYPE IF EXISTS %I CASCADE', r.typname);
    END LOOP;

    -- Drop extensions
    DROP EXTENSION IF EXISTS pg_trgm;
    DROP EXTENSION IF EXISTS pgcrypto;
END $$;
