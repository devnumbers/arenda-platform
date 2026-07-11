CREATE INDEX IF NOT EXISTS idx_login_codes_phone_purpose ON login_codes (phone, purpose) WHERE used = false;
CREATE INDEX IF NOT EXISTS idx_login_codes_email_purpose ON login_codes (email, purpose) WHERE used = false;
CREATE INDEX IF NOT EXISTS idx_login_codes_phone_email_purpose ON login_codes(phone, email, purpose) WHERE used = false;
