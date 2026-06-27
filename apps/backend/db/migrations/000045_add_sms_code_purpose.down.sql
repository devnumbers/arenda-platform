DROP INDEX IF EXISTS idx_sms_codes_phone_purpose;
ALTER TABLE sms_codes DROP COLUMN IF EXISTS purpose;
