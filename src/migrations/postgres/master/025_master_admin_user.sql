-- 025_master_admin_user.sql
-- Admin users table for root-level access (managing tenants)
-- Applied to master database (sc-master)

BEGIN;

CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE IF NOT EXISTS admin_user(
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email citext UNIQUE NOT NULL,
  name text,
  level int NOT NULL DEFAULT 100,        -- 100 = root level
  status text NOT NULL DEFAULT 'active',  -- 'active', 'inactive', 'suspended'
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_admin_user_email ON admin_user(email);
CREATE INDEX IF NOT EXISTS ix_admin_user_status ON admin_user(status) WHERE status != 'active';

COMMENT ON TABLE admin_user IS 'Root-level admin users for managing tenants';
COMMENT ON COLUMN admin_user.level IS 'Access level (always 100 for root admins)';
COMMENT ON COLUMN admin_user.status IS 'Admin status: active, inactive, suspended';

COMMIT;

