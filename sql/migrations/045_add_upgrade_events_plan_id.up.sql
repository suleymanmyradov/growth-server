-- Add plan_id FK to upgrade_events for referential integrity while keeping plan_code for audit immutability
ALTER TABLE upgrade_events ADD COLUMN plan_id UUID REFERENCES plans(id) ON DELETE SET NULL;

-- Backfill plan_id from existing plan_code values
UPDATE upgrade_events ue
SET plan_id = p.id
FROM plans p
WHERE p.code = ue.plan_code AND ue.plan_code IS NOT NULL;
