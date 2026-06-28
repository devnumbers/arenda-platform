-- name: GetUserByID :one
SELECT id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted FROM users WHERE id = $1;

-- name: GetUserByIDForUpdate :one
SELECT id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted FROM users WHERE id = $1 FOR UPDATE;

-- name: GetUserByPhone :one
SELECT id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted FROM users WHERE phone = $1;

-- name: CreateUser :one
INSERT INTO users (id, phone, role, phone_encrypted) VALUES ($1, $2, $3, $4)
ON CONFLICT (phone) DO UPDATE SET phone = EXCLUDED.phone, phone_encrypted = EXCLUDED.phone_encrypted
RETURNING id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted;

-- name: GetLatestSMSCodeByPhoneAndPurpose :one
SELECT id, user_id, phone, code_hash, expires_at, used, created_at, purpose, phone_encrypted FROM sms_codes
WHERE phone = $1 AND purpose = $2 AND used = false AND expires_at > $3
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE;

-- name: GetLatestSMSCodeByPhoneAndPurposeAndUserID :one
SELECT id, user_id, phone, code_hash, expires_at, used, created_at, purpose, phone_encrypted FROM sms_codes
WHERE phone = $1 AND purpose = $2 AND user_id = $3 AND used = false AND expires_at > $4
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE;

-- name: CreateSMSCode :exec
INSERT INTO sms_codes (id, phone, code_hash, expires_at, user_id, purpose, phone_encrypted)
VALUES ($1, $2, $3, $4, $5, $6, $7);

-- name: MarkSMSCodeUsed :exec
UPDATE sms_codes SET used = true WHERE id = $1;

-- name: DeleteSMSCodeByID :exec
DELETE FROM sms_codes WHERE id = $1;

-- name: DeleteSMSCodeByPhoneAndPurpose :exec
DELETE FROM sms_codes WHERE phone = $1 AND purpose = $2;

-- name: DeleteSMSCodesByUserID :exec
DELETE FROM sms_codes WHERE user_id = $1;

-- name: DeleteExpiredSMSCodes :exec
DELETE FROM sms_codes WHERE expires_at < $1;

-- name: DeleteExpiredSMSCodesBatch :execrows
DELETE FROM sms_codes t WHERE t.ctid IN (
    SELECT s.ctid FROM sms_codes s WHERE s.expires_at < $1 LIMIT $2
);

-- name: GetLoginAttemptByPhone :one
SELECT id, phone, failures, first_failure_at, last_failure_at, user_id, phone_encrypted FROM login_attempts WHERE phone = $1;

-- name: UpsertLoginAttempt :exec
INSERT INTO login_attempts (phone, failures, first_failure_at, last_failure_at, user_id, phone_encrypted)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (phone) DO UPDATE SET
    failures = EXCLUDED.failures,
    first_failure_at = EXCLUDED.first_failure_at,
    last_failure_at = EXCLUDED.last_failure_at,
    user_id = EXCLUDED.user_id,
    phone_encrypted = EXCLUDED.phone_encrypted;

-- name: DeleteLoginAttemptByPhone :exec
DELETE FROM login_attempts WHERE phone = $1;

-- name: DeleteLoginAttemptsByUserID :exec
DELETE FROM login_attempts WHERE user_id = $1;

-- name: DeleteStaleLoginAttempts :exec
DELETE FROM login_attempts WHERE last_failure_at < $1;

-- name: DeleteStaleLoginAttemptsBatch :execrows
DELETE FROM login_attempts t WHERE t.ctid IN (
    SELECT a.ctid FROM login_attempts a WHERE a.last_failure_at < $1 LIMIT $2
);

-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, expires_at, last_used_at)
VALUES ($1, $2, $3, $4) RETURNING id, user_id, token_hash, expires_at, created_at, last_used_at;

-- name: UpdateSession :exec
UPDATE sessions SET expires_at = $1, last_used_at = $2 WHERE token_hash = $3;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteSessionsByUserID :exec
DELETE FROM sessions WHERE user_id = $1;

-- name: DeleteSessionsByUserIDExcept :exec
DELETE FROM sessions WHERE user_id = $1 AND token_hash <> $2;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < $1;

-- name: DeleteExpiredSessionsBatch :execrows
DELETE FROM sessions t WHERE t.ctid IN (
    SELECT s.ctid FROM sessions s WHERE s.expires_at < $1 LIMIT $2
);

-- name: GetSessionByTokenHash :one
SELECT s.id, s.token_hash, s.expires_at, s.created_at, s.last_used_at,
       u.id AS user_id, u.phone, u.role, u.name, u.surname, u.patronymic, u.email, u.phone_encrypted
FROM sessions s
JOIN users u ON s.user_id = u.id
WHERE s.token_hash = $1 AND s.expires_at > $2;

-- name: GetUserPhoneByID :one
SELECT phone, phone_encrypted FROM users WHERE id = $1;

-- name: UpdateUser :one
UPDATE users
SET name = $2,
    surname = $3,
    patronymic = $4,
    email = $5,
    updated_at = now()
WHERE id = $1
RETURNING id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted;

-- name: UpdateUserPhone :one
UPDATE users
SET phone = $2,
    phone_encrypted = $3,
    updated_at = now()
WHERE id = $1
RETURNING id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted;
