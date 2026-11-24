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

-- Hot-path lookups: middleware resolves users by id. Partial indexes keep
-- soft-deleted rows out of the index entirely.
CREATE INDEX IF NOT EXISTS idx_users_active ON users (id) WHERE flag_use_status = 1;
