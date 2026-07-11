CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_login_attempts_last_failure_at ON login_attempts(last_failure_at);
