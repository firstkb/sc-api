-- 000_master.sql
-- Master database schema
-- PostgreSQL 15+

BEGIN;


-- ──────────────────────────────────────────────────────────────────────────────
-- Types
-- ──────────────────────────────────────────────────────────────────────────────
DO $$ BEGIN
  CREATE TYPE plan_t AS ENUM ('trial','light','pro','enterprise');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

DO $$ BEGIN
  CREATE TYPE isolation_mode AS ENUM ('sandbox','dedicated_schema','dedicated_db');
EXCEPTION WHEN duplicate_object THEN NULL; END $$;

-- ──────────────────────────────────────────────────────────────────────────────
-- Generic updated_at trigger
-- ──────────────────────────────────────────────────────────────────────────────
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger AS $$
BEGIN
  NEW.updated_at := now();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- ──────────────────────────────────────────────────────────────────────────────
-- DB instances registry
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS db_instance (
  id           BIGSERIAL PRIMARY KEY,
  code         text UNIQUE NOT NULL,                 -- e.g., 'S1', 'S2'
  dns          text,                                 -- full DSN for dev (optional)
  secret_name  text,                                 -- Secrets Manager name/ARN (optional)
  created_at   timestamptz NOT NULL DEFAULT now(),
  updated_at   timestamptz NOT NULL DEFAULT now(),
  CHECK (dns IS NOT NULL OR secret_name IS NOT NULL)
);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='trg_db_instance_updated_at') THEN
    CREATE TRIGGER trg_db_instance_updated_at
      BEFORE UPDATE ON db_instance
      FOR EACH ROW EXECUTE FUNCTION set_updated_at();
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS ix_db_instance_updated_at ON db_instance(updated_at);

-- ──────────────────────────────────────────────────────────────────────────────
-- Tenants (no plan column: plan is tracked in tenant_plan_history)
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS tenant (
  id          BIGINT GENERATED ALWAYS AS IDENTITY (START WITH 100 INCREMENT BY 1) PRIMARY KEY,
  name        text NOT NULL,
  isolation   isolation_mode NOT NULL DEFAULT 'sandbox',
  status      text NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended')),
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now()
);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='trg_tenant_updated_at') THEN
    CREATE TRIGGER trg_tenant_updated_at
      BEFORE UPDATE ON tenant
      FOR EACH ROW EXECUTE FUNCTION set_updated_at();
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS ix_tenant_updated_at ON tenant(updated_at);
CREATE INDEX IF NOT EXISTS ix_tenant_status      ON tenant(status);

-- ──────────────────────────────────────────────────────────────────────────────
-- Tenant domains (global unique, lowercase/punycode host)
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS tenant_domain (
  id          BIGSERIAL PRIMARY KEY,
  tenant_id   BIGINT NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  host        text NOT NULL,                         -- lowercase/punycode
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  CHECK (host = lower(host))
);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='trg_tenant_domain_updated_at') THEN
    CREATE TRIGGER trg_tenant_domain_updated_at
      BEFORE UPDATE ON tenant_domain
      FOR EACH ROW EXECUTE FUNCTION set_updated_at();
  END IF;
END $$;

CREATE INDEX IF NOT EXISTS ix_tenant_domain_host ON tenant_domain(host);
CREATE INDEX IF NOT EXISTS ix_tenant_domain_tenant     ON tenant_domain(tenant_id);
CREATE INDEX IF NOT EXISTS ix_tenant_domain_updated_at ON tenant_domain(updated_at);

-- ──────────────────────────────────────────────────────────────────────────────
-- Tenant DB bindings (one active row per tenant)
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS tenant_db (
  id             BIGSERIAL PRIMARY KEY,
  tenant_id      BIGINT NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  db_instance_id BIGINT NOT NULL REFERENCES db_instance(id) ON DELETE RESTRICT,
  db_name        text NOT NULL,
  created_at     timestamptz NOT NULL DEFAULT now(),
  updated_at     timestamptz NOT NULL DEFAULT now()
);

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_trigger WHERE tgname='trg_tenant_db_updated_at') THEN
    CREATE TRIGGER trg_tenant_db_updated_at
      BEFORE UPDATE ON tenant_db
      FOR EACH ROW EXECUTE FUNCTION set_updated_at();
  END IF;
END $$;

-- One active DB per tenant
CREATE UNIQUE INDEX IF NOT EXISTS ux_tenant_db_tenant_one
  ON tenant_db (tenant_id);

-- Avoid duplicates for same instance/name
CREATE UNIQUE INDEX IF NOT EXISTS ux_tenant_db_tenant_instance_name
  ON tenant_db (tenant_id, db_instance_id, db_name);

CREATE INDEX IF NOT EXISTS ix_tenant_db_instance   ON tenant_db(db_instance_id);
CREATE INDEX IF NOT EXISTS ix_tenant_db_updated_at ON tenant_db(updated_at);

-- ──────────────────────────────────────────────────────────────────────────────
-- Tenant plan history (temporal)
-- ──────────────────────────────────────────────────────────────────────────────
CREATE TABLE IF NOT EXISTS tenant_plan_history (
  id          BIGSERIAL PRIMARY KEY,
  tenant_id   BIGINT NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
  plan        plan_t NOT NULL,
  valid_from  timestamptz NOT NULL,
  valid_to    timestamptz,
  created_at  timestamptz NOT NULL DEFAULT now()
);

-- 1) open intervals (current plan without valid_to)
CREATE INDEX IF NOT EXISTS ix_tenant_plan_open
  ON tenant_plan_history (tenant_id, valid_from DESC)
  WHERE valid_to IS NULL;

-- 2) closed intervals (with valid_to)
CREATE INDEX IF NOT EXISTS ix_tenant_plan_closed
  ON tenant_plan_history (tenant_id, valid_from DESC, valid_to)
  WHERE valid_to IS NOT NULL;

CREATE OR REPLACE VIEW v_tenant_current_plan AS
SELECT DISTINCT ON (tenant_id)
  tenant_id, plan, valid_from, valid_to
FROM tenant_plan_history
WHERE valid_from <= now() AND (valid_to IS NULL OR now() < valid_to)
ORDER BY tenant_id, valid_from DESC;

-- ──────────────────────────────────────────────────────────────────────────────
-- Resolver view: host → tenant + active DB + instance + current plan
-- ──────────────────────────────────────────────────────────────────────────────
CREATE OR REPLACE VIEW v_tenant_by_host AS
SELECT
  td.host                          AS host,
  t.id                             AS tenant_id,
  t.name                           AS tenant_name,
  cp.plan                          AS plan,                 -- from history
  t.isolation,
  t.status,
  t.updated_at                     AS tenant_updated_at,
  tdb.db_name,
  di.id                            AS db_instance_id,
  di.code                          AS db_instance_code,
  di.dns,
  di.secret_name,
  GREATEST(td.updated_at, tdb.updated_at, t.updated_at, di.updated_at) AS updated_at
FROM tenant_domain td
JOIN tenant t        ON t.id = td.tenant_id
JOIN tenant_db tdb   ON tdb.tenant_id = t.id
JOIN db_instance di  ON di.id = tdb.db_instance_id
LEFT JOIN v_tenant_current_plan cp ON cp.tenant_id = t.id;

-- ──────────────────────────────────────────────────────────────────────────────
-- Seed helper (optional): upsert S1 using a literal DSN for dev.
-- Replace '<MASTER_DSN>' with your actual master DSN if you want to seed via SQL.
-- In production prefer bootstrapping from the app using ENV to avoid secrets in DDL.
--
-- INSERT INTO db_instance(code, dns)
-- VALUES ('S1', '<MASTER_DSN>')
-- ON CONFLICT (code) DO NOTHING;
--
-- Example to bind a tenant to S1 as default:
-- INSERT INTO tenant_db(tenant_id, db_instance_id, db_name)
-- SELECT $TENANT_ID, id, '<DB_NAME>' FROM db_instance WHERE code='S1'
-- ON CONFLICT (tenant_id)
-- DO UPDATE SET db_name = EXCLUDED.db_name, db_instance_id = EXCLUDED.db_instance_id, updated_at = now();
--
-- Or do this from the app at bootstrap time.
-- ──────────────────────────────────────────────────────────────────────────────

COMMIT;