DROP INDEX IF EXISTS idx_login_attempts_user_id;
ALTER TABLE login_attempts DROP COLUMN IF EXISTS user_id;
