-- 023_auth_otp_and_refresh.sql
-- OTP codes and refresh tokens for authentication
-- Applied to app databases (sc-app, sc-first, etc.)

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
