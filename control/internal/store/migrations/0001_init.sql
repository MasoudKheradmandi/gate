-- Corvus config store — initial schema. Portable SQL (SQLite now, PostgreSQL later).

CREATE TABLE users (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    email         TEXT NOT NULL UNIQUE,
    name          TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    role          TEXT NOT NULL DEFAULT 'admin',
    created_at    TIMESTAMP NOT NULL,
    last_login_at TIMESTAMP
);

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    ip         TEXT NOT NULL DEFAULT '',
    user_agent TEXT NOT NULL DEFAULT ''
);

CREATE TABLE certificates (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL,
    kind       TEXT NOT NULL,              -- selfsigned | custom | acme
    domains    TEXT NOT NULL DEFAULT '[]', -- JSON array (from the certificate SANs)
    not_after  TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL
);

CREATE TABLE waitrooms (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL UNIQUE,       -- slug used in URLs, cookies and Redis keys
    enabled    INTEGER NOT NULL DEFAULT 1,
    capacity   INTEGER NOT NULL,
    active_ttl INTEGER NOT NULL,
    grant_ttl  INTEGER NOT NULL,
    queue_ttl  INTEGER NOT NULL,
    refresh    INTEGER NOT NULL,
    admit_rate INTEGER NOT NULL DEFAULT 0,
    fail_mode  TEXT NOT NULL DEFAULT 'open',
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE proxy_hosts (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    domains         TEXT NOT NULL,         -- JSON array
    forward_scheme  TEXT NOT NULL DEFAULT 'http',
    forward_host    TEXT NOT NULL,
    forward_port    INTEGER NOT NULL,
    websocket       INTEGER NOT NULL DEFAULT 0,
    certificate_id  INTEGER REFERENCES certificates(id) ON DELETE SET NULL,
    ssl_forced      INTEGER NOT NULL DEFAULT 0,
    http2           INTEGER NOT NULL DEFAULT 1,
    waitroom_id     INTEGER REFERENCES waitrooms(id) ON DELETE SET NULL,
    waitroom_path   TEXT NOT NULL DEFAULT '/',
    advanced_config TEXT NOT NULL DEFAULT '',
    enabled         INTEGER NOT NULL DEFAULT 1,
    created_at      TIMESTAMP NOT NULL,
    updated_at      TIMESTAMP NOT NULL
);

-- Key/value metadata. config_rev increases on every configuration change.
CREATE TABLE meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
INSERT INTO meta(key, value) VALUES ('config_rev', '0');

CREATE TABLE config_versions (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at  TIMESTAMP NOT NULL,
    created_by  INTEGER REFERENCES users(id) ON DELETE SET NULL,
    status      TEXT NOT NULL,             -- active | superseded | failed
    message     TEXT NOT NULL DEFAULT '',
    test_output TEXT NOT NULL DEFAULT '',
    rollback_of INTEGER,
    dir         TEXT NOT NULL DEFAULT '',  -- generated config directory name
    config_rev  INTEGER NOT NULL DEFAULT 0 -- meta.config_rev this version was rendered from
);

CREATE TABLE audit_log (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    ts      TIMESTAMP NOT NULL,
    user_id INTEGER REFERENCES users(id) ON DELETE SET NULL,
    action  TEXT NOT NULL,
    target  TEXT NOT NULL DEFAULT '',
    detail  TEXT NOT NULL DEFAULT '{}',
    ip      TEXT NOT NULL DEFAULT ''
);
CREATE INDEX audit_log_ts ON audit_log(ts);
