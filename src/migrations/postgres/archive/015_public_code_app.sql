-- 015_public_code_app.sql
-- Public code table in app databases (sc-app, sc-first, etc.)
-- Codes are stored in tenant's app database for data isolation
-- IMPORTANT: This migration MUST be executed in app databases (sc-app, sc-first, etc.)

CREATE TABLE IF NOT EXISTS public_code (
  code text PRIMARY KEY,
  tenant_id BIGINT NOT NULL,
  resource_id uuid,
  expires_at timestamptz,
  metadata jsonb,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_public_code_tenant ON public_code(tenant_id);
CREATE INDEX IF NOT EXISTS idx_public_code_expires ON public_code(expires_at) WHERE expires_at IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_public_code_resource ON public_code(resource_id) WHERE resource_id IS NOT NULL;

COMMENT ON TABLE public_code IS 'Public codes stored in tenant app database for data isolation';
COMMENT ON COLUMN public_code.code IS 'Public identifier (e.g., survey code)';
COMMENT ON COLUMN public_code.tenant_id IS 'Tenant that owns this code';
COMMENT ON COLUMN public_code.resource_id IS 'Optional resource identifier (e.g., survey ID)';
COMMENT ON COLUMN public_code.expires_at IS 'Optional expiration time for temporary codes';
COMMENT ON COLUMN public_code.metadata IS 'Additional metadata (JSON)';
