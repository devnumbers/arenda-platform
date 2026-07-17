-- UUIDv7 rollout, phase 1: the application generates ids (uuid.NewV7()),
-- so the database no longer assigns DEFAULT gen_random_uuid() to id columns.
-- Data is untouched; only column defaults are dropped.
ALTER TABLE users ALTER COLUMN id DROP DEFAULT;
ALTER TABLE login_attempts ALTER COLUMN id DROP DEFAULT;
ALTER TABLE sessions ALTER COLUMN id DROP DEFAULT;
ALTER TABLE tariffs ALTER COLUMN id DROP DEFAULT;
ALTER TABLE user_subscriptions ALTER COLUMN id DROP DEFAULT;
ALTER TABLE properties ALTER COLUMN id DROP DEFAULT;
ALTER TABLE tenant_contacts ALTER COLUMN id DROP DEFAULT;
ALTER TABLE leases ALTER COLUMN id DROP DEFAULT;
ALTER TABLE recurring_operations ALTER COLUMN id DROP DEFAULT;
ALTER TABLE operations ALTER COLUMN id DROP DEFAULT;
ALTER TABLE reminders ALTER COLUMN id DROP DEFAULT;
ALTER TABLE sent_sms_reminders ALTER COLUMN id DROP DEFAULT;
ALTER TABLE payment_methods ALTER COLUMN id DROP DEFAULT;
ALTER TABLE subscription_payments ALTER COLUMN id DROP DEFAULT;
ALTER TABLE property_photos ALTER COLUMN id DROP DEFAULT;
ALTER TABLE login_codes ALTER COLUMN id DROP DEFAULT;
ALTER TABLE sent_email_reminders ALTER COLUMN id DROP DEFAULT;
ALTER TABLE operation_categories ALTER COLUMN id DROP DEFAULT;
