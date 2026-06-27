ALTER TABLE login_attempts ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_login_attempts_user_id ON login_attempts(user_id);
