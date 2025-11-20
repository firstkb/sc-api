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


