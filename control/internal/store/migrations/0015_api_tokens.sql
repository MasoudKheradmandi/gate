-- API tokens (Bearer cvs_…) for automation and Prometheus scraping.
-- Only the SHA-256 of the full token is stored; prefix is the first 8 characters for display.
CREATE TABLE api_tokens (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    name         TEXT NOT NULL,
    prefix       TEXT NOT NULL,
    token_hash   TEXT NOT NULL UNIQUE,
    role         TEXT NOT NULL,              -- admin | operator | viewer (≤ creator's role)
    created_by   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at   TIMESTAMP NOT NULL,
    last_used_at TIMESTAMP,
    expires_at   TIMESTAMP
);
CREATE INDEX api_tokens_created_by ON api_tokens(created_by);
