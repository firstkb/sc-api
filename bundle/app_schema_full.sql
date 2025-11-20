-- ============================================
-- Golden Schema Bundle: app_schema_full.sql
-- ============================================
--
-- This file contains all app database migrations
-- concatenated in order for fast database provisioning.
--
-- Generated: 2025-11-19T16:19:12-05:00
-- Source: migrations from archive/ and app/ directories
--
-- Usage:
--   1. CREATE DATABASE new_db;
--   2. psql -d new_db -f bundle/app_schema_full.sql
--   3. Run migrations to catch up if bundle is behind HEAD
--
-- Checksum: 07530f1abee9e959bff4efe559b88ecd9338c3ce18d581fe07009e0c8a5b3fd9
--
-- Migrations included:
--   - 010_app_template.sql (archive)
--   - 011_rls.sql (archive)
--   - 012_email_ci.sql (archive)
--   - 015_public_code_app.sql (archive)
--   - 020_idempotency.sql (archive)
--   - 022_notification_templates.sql (archive)
--   - 023_auth_otp_and_refresh.sql (archive)
--   - 024_membership_level.sql (archive)
--   - 026_event_log.sql (archive)
-- ============================================


-- ============================================
-- Migration: 010_app_template.sql (archive)
-- ============================================

-- 010_app_template.sql
-- App schema template (applied to public sandbox or per-tenant schema)

BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TABLE IF NOT EXISTS contacts (
  id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id    BIGINT NOT NULL,
  name         text NOT NULL,
  email        text,
  phone        text,
  tags         text[],
  created_at   timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_contacts_tenant ON contacts(tenant_id);
CREATE INDEX IF NOT EXISTS idx_contacts_email ON contacts(tenant_id, email);

CREATE TABLE IF NOT EXISTS users (
  id             uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id      BIGINT NOT NULL,
  email          text NOT NULL,
  phone          text,
  password_hash  text,
  active         boolean NOT NULL DEFAULT true,
  created_at     timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_users_email ON users(tenant_id, email);
CREATE INDEX IF NOT EXISTS idx_users_phone ON users(tenant_id, phone);

COMMIT;




-- ============================================
-- Migration: 011_rls.sql (archive)
-- ============================================

-- 011_rls.sql
-- Enable row level security for sandbox tables

BEGIN;

ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS p_users_tenant ON users;
CREATE POLICY p_users_tenant ON users
  USING (tenant_id = current_setting('app.tenant_id', true)::bigint)
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::bigint);

ALTER TABLE contacts ENABLE ROW LEVEL SECURITY;
ALTER TABLE contacts FORCE ROW LEVEL SECURITY;
DROP POLICY IF EXISTS p_contacts_tenant ON contacts;
CREATE POLICY p_contacts_tenant ON contacts
  USING (tenant_id = current_setting('app.tenant_id', true)::bigint)
  WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::bigint);

--ALTER TABLE public_code ENABLE ROW LEVEL SECURITY;
--ALTER TABLE public_code FORCE ROW LEVEL SECURITY;
--DROP POLICY IF EXISTS p_public_code_tenant ON public_code;
--CREATE POLICY p_public_code_tenant ON public_code
--  USING (tenant_id = current_setting('app.tenant_id', true)::bigint)
--  WITH CHECK (tenant_id = current_setting('app.tenant_id', true)::bigint);

COMMIT;




-- ============================================
-- Migration: 012_email_ci.sql (archive)
-- ============================================

-- 012_email_ci.sql
-- Convert users.email to citext and enforce case-insensitive uniqueness

BEGIN;

CREATE EXTENSION IF NOT EXISTS citext;

ALTER TABLE users
  ALTER COLUMN email TYPE citext
  USING email::citext;

DROP INDEX IF EXISTS ux_users_email;
DROP INDEX IF EXISTS ux_users_tenant_email;
DROP INDEX IF EXISTS ux_users_tenant_email_lower;
CREATE UNIQUE INDEX IF NOT EXISTS ux_users_tenant_email_ci ON users(tenant_id, email);

COMMIT;




-- ============================================
-- Migration: 015_public_code_app.sql (archive)
-- ============================================

-- 015_public_code_app.sql
-- Public code table in app databases (sc-app, sc-first, etc.)
-- Codes are stored in tenant's app database for data isolation
-- IMPORTANT: This migration MUST be executed in app databases (sc-app, sc-first, etc.)

BEGIN;

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

COMMIT;




-- ============================================
-- Migration: 020_idempotency.sql (archive)
-- ============================================

-- 020_idempotency.sql
-- Idempotency keys table for preventing duplicate POST requests
-- IMPORTANT: This migration MUST be executed in app databases (sc-app, sc-first, etc.), NOT in master

BEGIN;

CREATE TABLE IF NOT EXISTS idempotency_keys (
  tenant_id    BIGINT NOT NULL,
  key          TEXT   NOT NULL,
  route        TEXT   NOT NULL,
  body_sha256  TEXT   NOT NULL,
  actor_sub    UUID   NULL,     -- для Tier 3; для Tier 2 может быть NULL
  status       TEXT   NOT NULL CHECK (status IN ('started','succeeded','failed')),
  response     JSONB  NULL,     -- опционально: короткая кэшируемая форма ответа
  created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  ttl_at       TIMESTAMPTZ NOT NULL,
  PRIMARY KEY (tenant_id, key)
);

CREATE INDEX IF NOT EXISTS ix_idem_ttl ON idempotency_keys(ttl_at);
CREATE INDEX IF NOT EXISTS ix_idem_tenant_route ON idempotency_keys(tenant_id, route);

COMMENT ON TABLE idempotency_keys IS 'Idempotency keys for preventing duplicate POST requests';
COMMENT ON COLUMN idempotency_keys.key IS 'Idempotency-Key header value from client';
COMMENT ON COLUMN idempotency_keys.route IS 'HTTP route path (e.g., /survey/abc123/answer)';
COMMENT ON COLUMN idempotency_keys.body_sha256 IS 'SHA256 hash of request body for fingerprint matching';
COMMENT ON COLUMN idempotency_keys.actor_sub IS 'Actor subject (user ID) from JWT for Tier 3, NULL for Tier 2';
COMMENT ON COLUMN idempotency_keys.status IS 'Request status: started, succeeded, or failed';
COMMENT ON COLUMN idempotency_keys.response IS 'Cached response body (JSON) for successful requests';
COMMENT ON COLUMN idempotency_keys.ttl_at IS 'Expiration time for automatic cleanup';

COMMIT;




-- ============================================
-- Migration: 022_notification_templates.sql (archive)
-- ============================================

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



-- ============================================
-- Migration: 023_auth_otp_and_refresh.sql (archive)
-- ============================================

-- 023_auth_otp_and_refresh.sql
-- OTP codes and refresh tokens for authentication
-- Applied to app databases (sc-app, sc-first, etc.)

BEGIN;

-- OTP codes table
CREATE TABLE IF NOT EXISTS auth_otp(
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id BIGINT NOT NULL,          -- BIGINT for compatibility with existing tables
  channel text NOT NULL CHECK (channel IN ('email','sms')),
  address text NOT NULL,              -- email address or phone number
  code_hash text NOT NULL,            -- hashed OTP code (Argon2id or bcrypt)
  expires_at timestamptz NOT NULL,
  attempts int NOT NULL DEFAULT 0,    -- number of verification attempts
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_auth_otp_tenant_addr ON auth_otp(tenant_id, address);
CREATE INDEX IF NOT EXISTS ix_auth_otp_expires ON auth_otp(expires_at);

COMMENT ON TABLE auth_otp IS 'One-time password codes for authentication';
COMMENT ON COLUMN auth_otp.channel IS 'Delivery channel: email or sms';
COMMENT ON COLUMN auth_otp.address IS 'Email address or phone number (E.164 format)';
COMMENT ON COLUMN auth_otp.code_hash IS 'Hashed OTP code (never store plain text)';
COMMENT ON COLUMN auth_otp.attempts IS 'Number of failed verification attempts';

-- Refresh tokens table
CREATE TABLE IF NOT EXISTS auth_refresh_token(
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id BIGINT NOT NULL,          -- BIGINT for compatibility
  user_id uuid NOT NULL,
  token_hash text NOT NULL,            -- hashed refresh token (SHA256)
  client_id text NULL,                 -- optional client identifier
  device_id text NULL,                 -- optional device identifier
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz NULL          -- NULL if active, timestamp if revoked
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_auth_refresh_token_hash ON auth_refresh_token(token_hash);
CREATE INDEX IF NOT EXISTS ix_auth_refresh_token_user ON auth_refresh_token(tenant_id, user_id);
CREATE INDEX IF NOT EXISTS ix_auth_refresh_token_expires ON auth_refresh_token(expires_at);
CREATE INDEX IF NOT EXISTS ix_auth_refresh_token_revoked ON auth_refresh_token(revoked_at) WHERE revoked_at IS NOT NULL;

COMMENT ON TABLE auth_refresh_token IS 'Refresh tokens for JWT token rotation';
COMMENT ON COLUMN auth_refresh_token.token_hash IS 'SHA256 hash of the refresh token';
COMMENT ON COLUMN auth_refresh_token.revoked_at IS 'Timestamp when token was revoked (NULL if active)';

COMMIT;



-- ============================================
-- Migration: 024_membership_level.sql (archive)
-- ============================================

-- 024_membership_level.sql
-- Membership table with roles and access levels
-- Applied to app databases (sc-app, sc-first, etc.)

BEGIN;

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

COMMIT;



-- ============================================
-- Migration: 026_event_log.sql (archive)
-- ============================================

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



-- ============================================
-- Bundle End
-- ============================================
--
-- Total migrations: 9
-- Versions: 010_app_template, 011_rls, 012_email_ci, 015_public_code_app, 020_idempotency, 022_notification_templates, 023_auth_otp_and_refresh, 024_membership_level, 026_event_log
--
-- After applying this bundle, check schema_migrations table
-- and apply any additional migrations if needed.
-- ============================================
