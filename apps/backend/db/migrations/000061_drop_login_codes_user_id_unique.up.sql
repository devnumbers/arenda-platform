-- Drop the redundant unique index on (phone, purpose, user_id).
-- It conflicts with the email-based flow: the application deletes/creates codes
-- by (phone, email, purpose), so an unused code for the same phone+purpose+user_id
-- but a different email causes a duplicate-key violation on insert.
-- The remaining idx_login_codes_unique_unused on (phone, email, purpose) is sufficient.
DROP INDEX IF EXISTS idx_login_codes_phone_purpose_user_id_unused;
