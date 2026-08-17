-- DB-level CHECK constraints on data invariants that the domain cannot
-- guarantee for paths outside the application (seeds, manual edits, future
-- import paths). Format validation (phone/email) stays in the domain.
ALTER TABLE sessions ADD CONSTRAINT chk_sessions_expires_after_created
    CHECK (expires_at > created_at);

ALTER TABLE login_attempts ADD CONSTRAINT chk_login_attempts_failures_nonneg
    CHECK (failures >= 0);

ALTER TABLE login_attempts ADD CONSTRAINT chk_login_attempts_failure_order
    CHECK (first_failure_at <= last_failure_at);
