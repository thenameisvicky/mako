-- 005_governance_uniqueness.sql
-- One policy and one team-wide budget per team, so the dashboard can upsert
-- them by team id rather than juggling row ids.

CREATE UNIQUE INDEX IF NOT EXISTS idx_policies_team_unique
    ON policies (team_id) WHERE flag_use_status = 1;

-- A team-wide budget is the row with no user_id. Per-user budgets remain
-- possible and are unique per (team, user).
CREATE UNIQUE INDEX IF NOT EXISTS idx_budgets_team_unique
    ON budgets (team_id) WHERE flag_use_status = 1 AND user_id IS NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_budgets_team_user_unique
    ON budgets (team_id, user_id) WHERE flag_use_status = 1 AND user_id IS NOT NULL;
