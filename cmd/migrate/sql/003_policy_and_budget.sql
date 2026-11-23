-- 003_policy_and_budget.sql
-- Replaces the retired credential-swap feature: per-team policy and token budgets
-- evaluated on the hot path from cache, never from the database.

CREATE TABLE IF NOT EXISTS policies (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id           UUID        NOT NULL REFERENCES teams (id),
    allowed_models    TEXT[]      NOT NULL DEFAULT '{}',  -- empty = allow all
    blocked_models    TEXT[]      NOT NULL DEFAULT '{}',
    max_tokens_per_request INTEGER NOT NULL DEFAULT 0,    -- 0 = unlimited
    flag_use_status   SMALLINT    NOT NULL DEFAULT 1,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS budgets (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    team_id           UUID        NOT NULL REFERENCES teams (id),
    user_id           UUID        REFERENCES users (id),   -- NULL = team-wide budget
    period            TEXT        NOT NULL DEFAULT 'monthly',
    token_limit       BIGINT      NOT NULL DEFAULT 0,      -- 0 = unlimited
    flag_use_status   SMALLINT    NOT NULL DEFAULT 1,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Usage is appended by a background worker, never on the hot path.
CREATE TABLE IF NOT EXISTS usage_records (
    id                BIGSERIAL PRIMARY KEY,
    team_id           UUID        NOT NULL,
    user_id           UUID        NOT NULL,
    model             TEXT        NOT NULL DEFAULT '',
    input_tokens      INTEGER     NOT NULL DEFAULT 0,
    output_tokens     INTEGER     NOT NULL DEFAULT 0,
    occurred_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_policies_team  ON policies (team_id) WHERE flag_use_status = 1;
CREATE INDEX IF NOT EXISTS idx_budgets_team   ON budgets  (team_id) WHERE flag_use_status = 1;
CREATE INDEX IF NOT EXISTS idx_usage_rollup   ON usage_records (team_id, user_id, occurred_at);
