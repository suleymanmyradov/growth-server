-- Migration 046 now creates the unified constraint directly.
-- This migration cleans up any partial unique indexes that may have been created
-- by earlier versions of migration 046 before it was consolidated.

DROP INDEX IF EXISTS uniq_plan_adjustments_habit;
DROP INDEX IF EXISTS uniq_plan_adjustments_goal;
