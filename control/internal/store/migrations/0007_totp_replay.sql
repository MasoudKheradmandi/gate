-- H11: remember the last accepted TOTP time step per user in the database, so a code
-- cannot be replayed after a restart (or on another control-plane node).
ALTER TABLE users ADD COLUMN mfa_last_step INTEGER NOT NULL DEFAULT 0;
