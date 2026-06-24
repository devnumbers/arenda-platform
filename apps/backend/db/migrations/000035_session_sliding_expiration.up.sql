ALTER TABLE sessions ADD COLUMN last_used_at TIMESTAMPTZ;
UPDATE sessions SET last_used_at = created_at;
ALTER TABLE sessions ALTER COLUMN last_used_at SET NOT NULL;

CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);
