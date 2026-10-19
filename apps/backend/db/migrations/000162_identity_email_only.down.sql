-- Revert of 000162: recreate the phone columns/indices/CHECK in the forms of
-- 000001/000046/000048/000055/000063. Data is lost. Phone columns come back
-- nullable — a reverse NOT NULL on a non-empty table is impossible; the email
-- columns go back to nullable.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- login_attempts
DROP INDEX IF EXISTS login_attempts_email_key;
ALTER TABLE login_attempts DROP COLUMN email;
ALTER TABLE login_attempts ADD COLUMN phone TEXT UNIQUE;
ALTER TABLE login_attempts ADD COLUMN phone_encrypted BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE login_attempts ADD COLUMN user_id UUID REFERENCES users(id) ON DELETE CASCADE;
CREATE INDEX IF NOT EXISTS idx_login_attempts_user_id ON login_attempts(user_id);

-- login_codes
DROP INDEX IF EXISTS idx_login_codes_unique_unused;
ALTER TABLE login_codes ALTER COLUMN email DROP NOT NULL;
ALTER TABLE login_codes ADD COLUMN phone TEXT;
ALTER TABLE login_codes ADD COLUMN phone_encrypted BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE login_codes ADD CONSTRAINT login_codes_phone_or_email CHECK (phone IS NOT NULL OR email IS NOT NULL);
CREATE UNIQUE INDEX IF NOT EXISTS idx_login_codes_unique_unused
ON login_codes (phone, COALESCE(email, ''), purpose)
WHERE used = false;

-- users
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users ADD COLUMN phone TEXT UNIQUE;
ALTER TABLE users ADD COLUMN phone_encrypted BOOLEAN NOT NULL DEFAULT FALSE;
