-- Shared database functions used across multiple tables
-- This migration should run first to ensure functions exist before triggers are created

-- Extension for UUID generation
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

-- Shared trigger function for updating updated_at timestamps
-- Using CREATE OR REPLACE to ensure idempotency
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
