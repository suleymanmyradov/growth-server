-- Add pgvector extension for AI/semantic search capabilities (optional)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_available_extensions WHERE name = 'vector') THEN
        CREATE EXTENSION IF NOT EXISTS vector;
    ELSE
        RAISE NOTICE 'pgvector extension not available; skipping vector/embedding setup';
    END IF;
END $$;

-- Add embedding column to articles for RAG/semantic search
-- 1536 dimensions = OpenAI text-embedding-3-small / text-embedding-ada-002
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'vector') THEN
        ALTER TABLE articles ADD COLUMN IF NOT EXISTS embedding vector(1536);
    END IF;
END $$;

-- Create approximate nearest neighbor index using ivfflat
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_type WHERE typname = 'vector') THEN
        CREATE INDEX IF NOT EXISTS idx_articles_embedding ON articles USING ivfflat (embedding vector_cosine_ops)
        WITH (lists = 100);
    END IF;
END $$;

-- Add metadata JSONB column to articles for flexible AI-derived attributes
-- (e.g., keywords, topic clusters, sentiment scores) without schema changes
ALTER TABLE articles ADD COLUMN IF NOT EXISTS ai_metadata JSONB NOT NULL DEFAULT '{}'::jsonb;
CREATE INDEX IF NOT EXISTS idx_articles_ai_metadata ON articles USING GIN(ai_metadata);
