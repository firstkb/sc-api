-- 020_idempotency.sql
-- Idempotency keys table for preventing duplicate POST requests
-- IMPORTANT: This migration MUST be executed in app databases (sc-app, sc-first, etc.), NOT in master

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
