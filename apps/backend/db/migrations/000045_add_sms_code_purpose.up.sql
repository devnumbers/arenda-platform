ALTER TABLE sms_codes ADD COLUMN IF NOT EXISTS purpose VARCHAR(32) NOT NULL DEFAULT 'login';
CREATE INDEX IF NOT EXISTS idx_sms_codes_phone_purpose ON sms_codes(phone, purpose);
