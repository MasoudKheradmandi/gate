-- Baseline protection: request body inspection detects (log only) on every host by default.
UPDATE proxy_hosts SET body_inspection = 'log' WHERE body_inspection = 'off';
