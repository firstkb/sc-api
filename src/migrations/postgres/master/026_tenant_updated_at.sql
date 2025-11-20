-- 026_tenant_updated_at.sql
-- Add updated_at column to tenant for incremental cache loading

BEGIN;

ALTER TABLE tenant
  ADD COLUMN IF NOT EXISTS updated_at timestamptz NOT NULL DEFAULT now();

CREATE OR REPLACE FUNCTION tenant_set_updated_at()
RETURNS trigger AS $$
BEGIN
    -- always update timestamp on row change
    NEW.updated_at := now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER tenant_updated_at_trigger
BEFORE UPDATE ON tenant
FOR EACH ROW
EXECUTE FUNCTION tenant_set_updated_at();

COMMIT;


