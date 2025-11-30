-- 030_auth_tables.sql
-- Master DB tables for authentication workflow (OTP, refresh tokens, memberships)

CREATE TABLE IF NOT EXISTS auth_otp (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id  BIGINT NOT NULL,
  channel    text   NOT NULL,
  address    text   NOT NULL,
  code_hash  text   NOT NULL,
  expires_at timestamptz NOT NULL,
  attempts   integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS ix_auth_otp_lookup
  ON auth_otp (tenant_id, channel, address, created_at DESC);
CREATE INDEX IF NOT EXISTS ix_auth_otp_expires
  ON auth_otp (tenant_id, expires_at);

CREATE TABLE IF NOT EXISTS auth_refresh_token (
  id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id  BIGINT NOT NULL,
  user_id    uuid   NOT NULL,
  token_hash text   NOT NULL UNIQUE,
  client_id  text,
  device_id  text,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz
);

CREATE INDEX IF NOT EXISTS ix_refresh_token_user
  ON auth_refresh_token (tenant_id, user_id);
CREATE INDEX IF NOT EXISTS ix_refresh_token_expires
  ON auth_refresh_token (tenant_id, expires_at);
CREATE INDEX IF NOT EXISTS ix_refresh_token_revoked
  ON auth_refresh_token (tenant_id, revoked_at)
  WHERE revoked_at IS NULL;

