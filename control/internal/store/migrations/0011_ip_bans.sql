-- Automatic / manual IP bans enforced by the gateway (shared memory, pushed by corvus-control).
CREATE TABLE ip_bans (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    ip         TEXT NOT NULL UNIQUE,
    reason     TEXT NOT NULL DEFAULT '',
    source     TEXT NOT NULL DEFAULT 'auto',   -- auto | manual
    offenses   INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP,                      -- NULL = until removed
    created_by INTEGER REFERENCES users(id) ON DELETE SET NULL
);
CREATE INDEX ip_bans_expires ON ip_bans(expires_at);
