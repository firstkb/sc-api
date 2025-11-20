-- 001_meta_master.sql
-- Master metadata schema for multi-tenant routing & domains
-- PostgreSQL 15+

BEGIN;

-- For gen_random_uuid()
CREATE EXTENSION IF NOT EXISTS pgcrypto;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'isolation_mode') THEN
    CREATE TYPE isolation_mode AS ENUM ('sandbox','dedicated_schema','dedicated_db');
  END IF;
END $$;

CREATE TABLE IF NOT EXISTS tenant (
  id           BIGINT GENERATED ALWAYS AS IDENTITY (START WITH 100 INCREMENT BY 1),
  name         text NOT NULL,
  subdomain    text UNIQUE,
  is_sandbox   boolean NOT NULL DEFAULT false,
  isolation    isolation_mode NOT NULL DEFAULT 'sandbox',
  plan         text NOT NULL DEFAULT 'sandbox' CHECK (plan IN ('sandbox','pro','enterprise')),
  db_name      text NOT NULL,
  created_at   timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (id)
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_tenant_subdomain ON tenant(subdomain);

CREATE TABLE IF NOT EXISTS tenant_domain (
  id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   BIGINT NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  host        text NOT NULL,
  is_default  boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX IF NOT EXISTS ux_tenant_domain_tenant_host ON tenant_domain(tenant_id, host);
CREATE UNIQUE INDEX IF NOT EXISTS ux_tenant_domain_host_default ON tenant_domain(host) WHERE is_default;

-- Helper view for lookups
CREATE OR REPLACE VIEW v_tenant_by_host AS
SELECT td.host,
       t.id          AS tenant_id,
       t.name        AS tenant_name,
       t.is_sandbox,
       t.isolation,
       t.db_name,
       t.plan,
       td.is_default
FROM tenant_domain td
JOIN tenant t ON t.id = td.tenant_id;

COMMIT;


