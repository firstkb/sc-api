-- 017_migration_runs.sql
-- Table for tracking migration runs across all app databases
-- IMPORTANT: This migration MUST be executed in master database (sc-master), NOT in app databases

BEGIN;

CREATE TABLE IF NOT EXISTS migration_runs (
  id           BIGSERIAL PRIMARY KEY,
  version      TEXT NOT NULL,           -- application version (from build)
  started_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  status       TEXT NOT NULL CHECK (status IN ('running', 'completed', 'failed')),
  error        TEXT,
  applied_dbs  TEXT[]                  -- list of databases where migrations were applied
);

-- Only one running migration per version
CREATE UNIQUE INDEX IF NOT EXISTS ux_migration_runs_running 
  ON migration_runs(version, status) 
  WHERE status = 'running';

-- Index for quick status lookups
CREATE INDEX IF NOT EXISTS idx_migration_runs_status 
  ON migration_runs(status, started_at DESC);

COMMENT ON TABLE migration_runs IS 'Tracks migration runs to prevent duplicates and monitor status';
COMMENT ON COLUMN migration_runs.version IS 'Application version (e.g., 1.0.0.dev)';
COMMENT ON COLUMN migration_runs.status IS 'Migration status: running, completed, or failed';
COMMENT ON COLUMN migration_runs.applied_dbs IS 'List of database names where migrations were applied';

COMMIT;

