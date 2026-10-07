SET lock_timeout = '1s';
SET statement_timeout = '5s';

ALTER TABLE payments DROP COLUMN notify_auto_paid;
