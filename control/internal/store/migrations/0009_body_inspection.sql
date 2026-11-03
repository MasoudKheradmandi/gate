-- Request body inspection per proxy host: off | log | block, with a size limit (KiB).
ALTER TABLE proxy_hosts ADD COLUMN body_inspection TEXT NOT NULL DEFAULT 'off';
ALTER TABLE proxy_hosts ADD COLUMN body_max_kb INTEGER NOT NULL DEFAULT 1024;
