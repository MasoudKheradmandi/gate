-- Security hardening (review 2026-11-03): waiting room flood limits, idle session expiry.
ALTER TABLE waitrooms ADD COLUMN max_queue INTEGER NOT NULL DEFAULT 100000;
ALTER TABLE waitrooms ADD COLUMN max_new_per_ip INTEGER NOT NULL DEFAULT 60;
ALTER TABLE waitrooms ADD COLUMN bind_ip INTEGER NOT NULL DEFAULT 0;
ALTER TABLE sessions ADD COLUMN last_seen_at TIMESTAMP;
