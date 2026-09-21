SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE push_subscriptions
    DROP COLUMN IF EXISTS enabled,
    DROP COLUMN IF EXISTS category_rental,
    DROP COLUMN IF EXISTS category_payments_operations,
    DROP COLUMN IF EXISTS category_tasks,
    DROP COLUMN IF EXISTS category_shared_access;

DROP TABLE IF EXISTS notification_email_preferences;
