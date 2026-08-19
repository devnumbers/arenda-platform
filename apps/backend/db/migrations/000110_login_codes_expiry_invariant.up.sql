SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- DB-level expiry invariant for login_codes: 000103 closed sessions and
-- login_attempts, login_codes stayed outside (schema audit #322, decision
-- #324). Same shape as chk_sessions_expires_after_created: a code valid the
-- instant it is created is meaningless, expiry must be strictly later.
ALTER TABLE login_codes ADD CONSTRAINT chk_login_codes_expires_after_created
    CHECK (expires_at > created_at);
