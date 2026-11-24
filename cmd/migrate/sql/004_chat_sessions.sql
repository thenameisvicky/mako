-- 004_chat_sessions.sql
-- Storage for /push: Claude Code transcripts uploaded from a machine,
-- encrypted at rest with AES-256-GCM and shared across a team.
--
-- transcript holds the full .jsonl file as a single EncryptField value. It is
-- never indexed or searched server-side; the proxy only needs to hand it back.

CREATE TABLE IF NOT EXISTS chat_sessions (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    session_id      TEXT        NOT NULL,   -- Claude Code session UUID
    user_id         UUID        NOT NULL REFERENCES users (id),
    team_id         UUID        NOT NULL REFERENCES teams (id),
    project_path    TEXT        NOT NULL,   -- encrypted: original project directory
    title           TEXT        NOT NULL DEFAULT '',
    transcript      TEXT        NOT NULL,   -- encrypted: raw .jsonl contents
    message_count   INTEGER     NOT NULL DEFAULT 0,
    size_bytes      INTEGER     NOT NULL DEFAULT 0,
    checksum        TEXT        NOT NULL DEFAULT '',
    synced_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    flag_use_status SMALLINT    NOT NULL DEFAULT 1,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Re-syncing the same session from the same machine updates in place.
CREATE UNIQUE INDEX IF NOT EXISTS idx_chat_sessions_unique
    ON chat_sessions (user_id, session_id) WHERE flag_use_status = 1;

-- Listing is always team-scoped, newest first.
CREATE INDEX IF NOT EXISTS idx_chat_sessions_team
    ON chat_sessions (team_id, synced_at DESC) WHERE flag_use_status = 1;
