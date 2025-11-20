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


