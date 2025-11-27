-- test_table.sql
-- test table for testing migrations

CREATE TABLE IF NOT EXISTS test_table (
  id          BIGSERIAL PRIMARY KEY,
  payload     JSONB,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
