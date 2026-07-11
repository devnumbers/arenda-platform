CREATE UNIQUE INDEX IF NOT EXISTS idx_login_codes_phone_purpose_user_id_unused
    ON login_codes (phone, purpose, user_id)
    WHERE used = false;
