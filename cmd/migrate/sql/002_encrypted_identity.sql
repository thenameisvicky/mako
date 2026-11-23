-- 002_encrypted_identity.sql
-- Move username / email / password to AES-256-GCM at rest.
--
-- AES-GCM uses a fresh nonce per encryption, so the ciphertext of a username is
-- different every time and cannot be matched with `WHERE username = $1`.
-- username_bidx holds a deterministic HMAC-SHA256 blind index of the username so
-- login can still find the row without storing it in the clear.
--
-- Additive only: existing plaintext usernames and bcrypt passwords keep working
-- and are migrated in place on the user's next successful login.

ALTER TABLE users ADD COLUMN IF NOT EXISTS username_bidx TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_users_username_bidx
    ON users (username_bidx) WHERE flag_use_status = 1 AND username_bidx IS NOT NULL;

-- Legacy plaintext lookup path, dropped once every row has been migrated.
CREATE INDEX IF NOT EXISTS idx_users_username_legacy
    ON users (username) WHERE flag_use_status = 1;
