SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- The grant is born on the current-address code check (verify-current), before
-- any new address is named; the address binds to the live grant at the next
-- step (request-new-email-code). Issue #1203, protocol #1202.
ALTER TABLE email_change_grants ALTER COLUMN email DROP NOT NULL;
