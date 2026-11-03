-- Sprint 3: rate limiting, client IP trust, exploit blocking.

CREATE TABLE rate_limits (
    id                  INTEGER PRIMARY KEY AUTOINCREMENT,
    name                TEXT NOT NULL UNIQUE,
    requests_per_second INTEGER NOT NULL,
    burst               INTEGER NOT NULL DEFAULT 0,
    max_connections     INTEGER NOT NULL DEFAULT 0,
    status_code         INTEGER NOT NULL DEFAULT 429,
    dry_run             INTEGER NOT NULL DEFAULT 0,
    exempt_cidrs        TEXT NOT NULL DEFAULT '[]',
    enabled             INTEGER NOT NULL DEFAULT 1,
    created_at          TIMESTAMP NOT NULL,
    updated_at          TIMESTAMP NOT NULL
);

ALTER TABLE proxy_hosts ADD COLUMN client_ip TEXT NOT NULL DEFAULT 'direct';
ALTER TABLE proxy_hosts ADD COLUMN rate_limit_id INTEGER REFERENCES rate_limits(id) ON DELETE SET NULL;
ALTER TABLE proxy_hosts ADD COLUMN block_exploits INTEGER NOT NULL DEFAULT 0;

-- Network settings (JSON) and the Cloudflare IP list snapshot (JSON).
INSERT INTO meta(key, value) VALUES ('network', '{"trusted_proxies":[],"real_ip_header":"X-Forwarded-For","mask_ips":false}');
INSERT INTO meta(key, value) VALUES ('cloudflare_ips', '');
