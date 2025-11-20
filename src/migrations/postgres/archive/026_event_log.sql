-- 026_event_log.sql
-- Event logging table for user events (auth, user actions, etc.)
-- Applied to app databases (sc-app, sc-first, etc.)

BEGIN;

CREATE TABLE IF NOT EXISTS event_log (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id BIGINT NOT NULL,
  user_id uuid NULL,                    -- NULL if event is not user-specific
  event_type text NOT NULL,             -- 'otp_request', 'otp_verify', 'login', 'logout', etc.
  event_data jsonb NULL,                 -- additional event data (JSON)
  ip_address inet NULL,                 -- client IP address
  user_agent text NULL,                 -- User-Agent header
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Indexes for fast lookups
CREATE INDEX IF NOT EXISTS ix_event_log_tenant_created ON event_log(tenant_id, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_event_log_user_created ON event_log(user_id, created_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS ix_event_log_type_created ON event_log(event_type, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_event_log_tenant_type_created ON event_log(tenant_id, event_type, created_at DESC);

-- RLS for tenant isolation
ALTER TABLE event_log ENABLE ROW LEVEL SECURITY;
ALTER TABLE event_log FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS p_event_log_tenant ON event_log;
CREATE POLICY p_event_log_tenant ON event_log
  USING (tenant_id = current_setting('app.tenant_id', true)::bigint)
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::bigint);

COMMENT ON TABLE event_log IS 'Event log for user actions and authentication events';
COMMENT ON COLUMN event_log.user_id IS 'User ID (NULL if event is not user-specific)';
COMMENT ON COLUMN event_log.event_type IS 'Event type: otp_request, otp_verify, login, logout, user_create, etc.';
COMMENT ON COLUMN event_log.event_data IS 'Additional event data as JSON (may contain masked PII)';
COMMENT ON COLUMN event_log.ip_address IS 'Client IP address for security tracking';

COMMIT;

