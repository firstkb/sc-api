-- 022_notification_templates.sql
-- Notification templates table for email/SMS templates with tenant-specific overrides
-- Applied to app databases (sc-app, sc-first, etc.)

BEGIN;

CREATE TABLE IF NOT EXISTS notification_template(
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id BIGINT NULL,              -- NULL = global template, BIGINT for compatibility
  kind text NOT NULL CHECK (kind IN ('email','sms')),
  key text NOT NULL,                  -- 'otp', 'high_risk', 'invite', etc.
  locale text NOT NULL DEFAULT 'default',
  subject text NULL,                   -- only for email
  html text NULL,                      -- email HTML body
  text text NULL,                      -- email text body & SMS
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(tenant_id, kind, key, locale)
);

CREATE INDEX IF NOT EXISTS ix_notification_template_tenant ON notification_template(tenant_id);
CREATE INDEX IF NOT EXISTS ix_notification_template_key ON notification_template(kind, key);

COMMENT ON TABLE notification_template IS 'Email/SMS notification templates with tenant-specific overrides';
COMMENT ON COLUMN notification_template.tenant_id IS 'NULL for global templates, tenant_id for tenant-specific overrides';
COMMENT ON COLUMN notification_template.kind IS 'Template type: email or sms';
COMMENT ON COLUMN notification_template.key IS 'Template key identifier (e.g., otp, high_risk, invite)';
COMMENT ON COLUMN notification_template.locale IS 'Locale code (e.g., en, ru, default)';

COMMIT;

