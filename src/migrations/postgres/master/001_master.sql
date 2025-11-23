    BEGIN;

    -- for case-insensitive e-mail
    CREATE EXTENSION IF NOT EXISTS citext;

    -- 1) Global identity subject
    CREATE TABLE IF NOT EXISTS identity_subject (
    id            BIGSERIAL PRIMARY KEY,
    type          TEXT NOT NULL DEFAULT 'email' CHECK (type IN ('email','phone')),     -- 'email' | 'phone'
    hmac          BYTEA NOT NULL,   -- HMAC(normalized value)
    hmac_kid      SMALLINT NOT NULL, -- key version (for rotation)
    is_primary    BOOLEAN NOT NULL DEFAULT false, -- optional UX hint
    status        TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active','blocked')),
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    UNIQUE (type, hmac)
    );

    -- 2) Identity subject tenant membership
    CREATE TABLE IF NOT EXISTS identity_tenant_membership (
    tenant_id   BIGINT NOT NULL REFERENCES tenant(id) ON DELETE CASCADE,
    identity_id BIGINT NOT NULL REFERENCES identity_subject(id) ON DELETE CASCADE,
    tenant_user_id TEXT NOT NULL, -- local user ID in tenant-DB
    role        TEXT   NOT NULL DEFAULT 'user',
    level       INT    NOT NULL DEFAULT 10,
    status      TEXT   NOT NULL DEFAULT 'active' CHECK (status IN ('active','blocked')),
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, identity_id, tenant_user_id)
    );

    CREATE INDEX IF NOT EXISTS ix_identity_tenant_membership_identity ON identity_tenant_membership(identity_id);
    CREATE INDEX IF NOT EXISTS ix_identity_tenant_membership_tenant ON identity_tenant_membership(tenant_id);
    CREATE INDEX IF NOT EXISTS ix_identity_tenant_membership_tenant_user ON identity_tenant_membership(tenant_user_id);
    
    COMMIT;