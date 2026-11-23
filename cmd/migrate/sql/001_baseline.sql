-- 001_baseline.sql
-- Baseline schema for the Mako proxy. Safe to run against an existing
-- database: every statement is idempotent.

CREATE TABLE IF NOT EXISTS teams (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name            TEXT        NOT NULL,
    flag_use_status SMALLINT    NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username        TEXT        NOT NULL,
    email           TEXT        NOT NULL,
    password        TEXT        NOT NULL,
    team_id         UUID        NOT NULL REFERENCES teams (id),
    role            TEXT        NOT NULL DEFAULT 'member',
    is_active       BOOLEAN     NOT NULL DEFAULT true,
    flag_use_status SMALLINT    NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS credentials (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID        NOT NULL REFERENCES users (id),
    team_id         UUID        NOT NULL REFERENCES teams (id),
    provider        TEXT        NOT NULL,
    ciphertext      TEXT        NOT NULL,
    nonce           TEXT        NOT NULL,
    flag_use_status SMALLINT    NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Hot-path lookups: middleware resolves users by id, proxy never touches the DB
-- once warm. Partial indexes keep soft-deleted rows out of the index entirely.
CREATE INDEX IF NOT EXISTS idx_users_active         ON users (id)                     WHERE flag_use_status = 1;
CREATE INDEX IF NOT EXISTS idx_credentials_lookup   ON credentials (user_id, provider) WHERE flag_use_status = 1;
