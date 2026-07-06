CREATE UNIQUE INDEX IF NOT EXISTS idx_login_codes_unique_unused
ON login_codes (phone, email, purpose)
WHERE used = false;
