-- RBAC: roles admin | operator | viewer (users.role already exists); account disable flag.
ALTER TABLE users ADD COLUMN disabled INTEGER NOT NULL DEFAULT 0;
