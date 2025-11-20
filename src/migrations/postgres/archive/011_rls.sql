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


