-- 024_membership_level.sql
-- Membership table with roles and access levels
-- Applied to app databases (sc-app, sc-first, etc.)

-- Create membership table if it doesn't exist
CREATE TABLE IF NOT EXISTS membership (
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  tenant_id BIGINT NOT NULL,
  role text NOT NULL DEFAULT 'user',      -- 'root', 'admin', 'manager', 'user', 'viewer'
  level int NOT NULL DEFAULT 40,         -- 100=root, 80=admin, 60=manager, 40=user, 20=viewer
  status text NOT NULL DEFAULT 'active',  -- 'active', 'inactive', 'suspended'
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, tenant_id)
);

-- Add columns if table already exists (for backward compatibility)
ALTER TABLE membership
  ADD COLUMN IF NOT EXISTS level int NOT NULL DEFAULT 40,
  ADD COLUMN IF NOT EXISTS role text NOT NULL DEFAULT 'user',
  ADD COLUMN IF NOT EXISTS status text NOT NULL DEFAULT 'active',
  ADD COLUMN IF NOT EXISTS created_at timestamptz NOT NULL DEFAULT now();

-- Create indexes
CREATE INDEX IF NOT EXISTS ix_membership_tenant_level ON membership(tenant_id, level);
CREATE INDEX IF NOT EXISTS ix_membership_user ON membership(user_id);
CREATE INDEX IF NOT EXISTS ix_membership_status ON membership(tenant_id, status) WHERE status != 'active';

COMMENT ON TABLE membership IS 'User memberships with roles and access levels per tenant';
COMMENT ON COLUMN membership.role IS 'Role name: root, admin, manager, user, viewer';
COMMENT ON COLUMN membership.level IS 'Access level: 100=root, 80=admin, 60=manager, 40=user, 20=viewer';
COMMENT ON COLUMN membership.status IS 'Membership status: active, inactive, suspended';
