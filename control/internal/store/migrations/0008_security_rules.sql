-- Security rules (Cloudflare-style custom rules): one ordered list, first block/allow match wins.
CREATE TABLE security_rules (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    name            TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    enabled         INTEGER NOT NULL DEFAULT 1,
    position        INTEGER NOT NULL,
    action          TEXT NOT NULL,                 -- block | allow | log
    status_code     INTEGER NOT NULL DEFAULT 403,
    skip_basic_auth INTEGER NOT NULL DEFAULT 0,
    scope           TEXT NOT NULL DEFAULT 'all',   -- all | hosts
    expression      TEXT NOT NULL,                 -- JSON {groups: [[condition...]...]}
    created_at      TIMESTAMP NOT NULL,
    updated_at      TIMESTAMP NOT NULL
);

CREATE TABLE security_rule_hosts (
    id      INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id INTEGER NOT NULL REFERENCES security_rules(id) ON DELETE CASCADE,
    host_id INTEGER NOT NULL REFERENCES proxy_hosts(id) ON DELETE CASCADE,
    UNIQUE (rule_id, host_id)
);

CREATE TABLE ip_lists (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT NOT NULL UNIQUE,
    description TEXT NOT NULL DEFAULT '',
    items       TEXT NOT NULL DEFAULT '[]',        -- JSON array of IPs/CIDRs
    created_at  TIMESTAMP NOT NULL,
    updated_at  TIMESTAMP NOT NULL
);
