ALTER TABLE users DROP COLUMN phone_encrypted;
ALTER TABLE sms_codes DROP COLUMN phone_encrypted;
ALTER TABLE login_attempts DROP COLUMN phone_encrypted;
