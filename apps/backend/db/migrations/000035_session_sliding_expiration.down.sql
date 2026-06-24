DROP INDEX IF EXISTS idx_sessions_expires_at;
ALTER TABLE sessions DROP COLUMN last_used_at;
