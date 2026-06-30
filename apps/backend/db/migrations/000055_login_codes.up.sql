CREATE TABLE IF NOT EXISTS login_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    phone TEXT,
    email TEXT,
    code_hash TEXT NOT NULL,
    purpose VARCHAR(32) NOT NULL DEFAULT 'login',
    expires_at TIMESTAMPTZ NOT NULL,
    used BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    phone_encrypted BOOLEAN NOT NULL DEFAULT false,
    CONSTRAINT login_codes_phone_or_email CHECK (phone IS NOT NULL OR email IS NOT NULL)
);

CREATE INDEX IF NOT EXISTS idx_login_codes_phone_purpose ON login_codes (phone, purpose) WHERE used = false;
CREATE INDEX IF NOT EXISTS idx_login_codes_email_purpose ON login_codes (email, purpose) WHERE used = false;
CREATE INDEX IF NOT EXISTS idx_login_codes_phone_email_purpose ON login_codes(phone, email, purpose) WHERE used = false;
CREATE INDEX IF NOT EXISTS idx_login_codes_expires_at ON login_codes (expires_at);
