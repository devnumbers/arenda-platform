SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE login_codes DROP CONSTRAINT IF EXISTS chk_login_codes_expires_after_created;
