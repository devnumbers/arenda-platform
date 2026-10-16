ALTER TABLE sessions DROP CONSTRAINT IF EXISTS chk_sessions_expires_after_created;
ALTER TABLE login_attempts DROP CONSTRAINT IF EXISTS chk_login_attempts_failures_nonneg;
ALTER TABLE login_attempts DROP CONSTRAINT IF EXISTS chk_login_attempts_failure_order;
