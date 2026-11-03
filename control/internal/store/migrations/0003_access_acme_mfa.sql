-- Sprint 4: access lists (NPM compatible), ACME certificates, administrator MFA.

CREATE TABLE access_lists (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT NOT NULL UNIQUE,
    satisfy    TEXT NOT NULL DEFAULT 'any',   -- any | all (IP rule AND/OR basic auth)
    allow      TEXT NOT NULL DEFAULT '[]',    -- JSON array of CIDRs/IPs
    deny       TEXT NOT NULL DEFAULT '[]',    -- JSON array of CIDRs/IPs or "all"
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);

CREATE TABLE access_list_users (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    access_list_id INTEGER NOT NULL REFERENCES access_lists(id) ON DELETE CASCADE,
    username       TEXT NOT NULL,
    password_hash  TEXT NOT NULL,              -- crypt(3) hash usable by nginx auth_basic
    UNIQUE (access_list_id, username)
);

ALTER TABLE proxy_hosts ADD COLUMN access_list_id INTEGER REFERENCES access_lists(id) ON DELETE SET NULL;

ALTER TABLE certificates ADD COLUMN status TEXT NOT NULL DEFAULT 'valid';     -- valid | pending | error
ALTER TABLE certificates ADD COLUMN last_error TEXT NOT NULL DEFAULT '';
ALTER TABLE certificates ADD COLUMN auto_renew INTEGER NOT NULL DEFAULT 0;
ALTER TABLE certificates ADD COLUMN acme TEXT NOT NULL DEFAULT '{}';          -- {email, challenge, dns_provider, staging, credentials(enc)}

CREATE TABLE acme_accounts (
    email        TEXT NOT NULL,
    staging      INTEGER NOT NULL,
    key_pem      TEXT NOT NULL,                -- encrypted with the data key
    registration TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (email, staging)
);

ALTER TABLE users ADD COLUMN totp_secret TEXT NOT NULL DEFAULT '';            -- encrypted with the data key
ALTER TABLE users ADD COLUMN mfa_enabled INTEGER NOT NULL DEFAULT 0;
ALTER TABLE users ADD COLUMN recovery_codes TEXT NOT NULL DEFAULT '[]';       -- JSON array of SHA-256 hashes
