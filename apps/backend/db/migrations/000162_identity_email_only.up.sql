-- Email-only identity (ADR 0067, карта #1525, спецификация тикета #1527):
-- телефон покидает Identity целиком, email — канонический идентификатор.
-- Append-only drop-миграция (прецеденты 000064/000108/000112): БД сбрасываема,
-- переноса данных нет — история решений читается по журналу миграций.
SET lock_timeout = '1s';
SET statement_timeout = '5s';

-- users: email становится каноническим идентификатором.
-- Строки без email в новой модели неоперабельны (входить некуда): это сид-админ
-- 000042 на реплее с нуля; решения владельца 18.10 — прод-пользователей нет,
-- переноса данных нет. Внешние ключи на users каскадные (000001).
DELETE FROM users WHERE email IS NULL;
-- Уникальность без регистра обеспечивает существующий users_email_lowercase_unique (000049).
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users DROP COLUMN phone;                -- уносит constraint users_phone_key
ALTER TABLE users DROP COLUMN phone_encrypted;      -- 000048

-- login_codes: триплет (phone, COALESCE(email,''), purpose) схлопывается в пару (email, purpose).
DROP INDEX IF EXISTS idx_login_codes_unique_unused;              -- форма 000063
ALTER TABLE login_codes DROP CONSTRAINT login_codes_phone_or_email;  -- 000055
ALTER TABLE login_codes ALTER COLUMN email SET NOT NULL;
ALTER TABLE login_codes DROP COLUMN phone;
ALTER TABLE login_codes DROP COLUMN phone_encrypted;             -- 000048
CREATE UNIQUE INDEX idx_login_codes_unique_unused ON login_codes (email, purpose) WHERE used = false;
-- idx_login_codes_expires_at и chk_login_codes_expires_after_created (000110) остаются;
-- login_codes.user_id остаётся nullable — код на новый адрес живёт до создания аккаунта.

-- login_attempts: ключ окна попыток — канонический email (research #1526: OWASP
-- «идентификатор + IP»; user_id мёртв после удаления протокола смены телефона).
DELETE FROM login_attempts;                  -- одноразовое состояние, не учётные данные; страховка воспроизводимости
ALTER TABLE login_attempts DROP COLUMN phone;            -- уносит phone UNIQUE
ALTER TABLE login_attempts DROP COLUMN phone_encrypted;  -- 000048
ALTER TABLE login_attempts DROP COLUMN user_id;          -- 000046; уносит idx_login_attempts_user_id
ALTER TABLE login_attempts ADD COLUMN email TEXT NOT NULL;
CREATE UNIQUE INDEX login_attempts_email_key ON login_attempts (email);
-- chk_login_attempts_failures_nonneg и chk_login_attempts_failure_order (000103)
-- и idx_login_attempts_last_failure_at (000062, cleaner) остаются без изменений.
