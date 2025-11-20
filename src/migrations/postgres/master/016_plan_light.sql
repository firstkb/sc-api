-- 016_plan_light.sql
-- Migration: Replace 'sandbox' plan with 'light' for better marketing
-- This is a data migration that updates existing tenants and schema constraint
-- IMPORTANT: This migration MUST be executed in master database (sc-master)

BEGIN;

-- Step 1: Update existing tenants with plan='sandbox' to plan='light'
UPDATE tenant 
SET plan = 'light' 
WHERE plan = 'sandbox';

-- Step 2: Update the CHECK constraint to allow 'light' instead of 'sandbox'
ALTER TABLE tenant DROP CONSTRAINT IF EXISTS tenant_plan_check;
ALTER TABLE tenant ADD CONSTRAINT tenant_plan_check 
    CHECK (plan IN ('light','pro','enterprise'));

-- Step 3: Update default value
ALTER TABLE tenant ALTER COLUMN plan SET DEFAULT 'light';

COMMIT;


