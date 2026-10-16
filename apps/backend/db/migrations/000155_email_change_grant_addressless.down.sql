SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- Откат 000155: безадресные гранты — переходящее состояние флоу (TTL ≈10 мин),
-- они удаляются; привязанные гранты восстанавливают исходную форму колонки.
DELETE FROM email_change_grants WHERE email IS NULL;

ALTER TABLE email_change_grants ALTER COLUMN email SET NOT NULL;
