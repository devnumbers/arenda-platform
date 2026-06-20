-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetUserByPhone :one
SELECT * FROM users WHERE phone = $1;

-- name: CreateUser :one
INSERT INTO users (id, phone, role) VALUES ($1, $2, $3)
ON CONFLICT (phone) DO UPDATE SET phone = EXCLUDED.phone
RETURNING *;

-- name: GetLatestSMSCodeByPhone :one
SELECT * FROM sms_codes
WHERE phone = $1 AND used = false AND expires_at > $2
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE;

-- name: CreateSMSCode :one
INSERT INTO sms_codes (id, phone, code_hash, expires_at)
VALUES ($1, $2, $3, $4) RETURNING *;

-- name: MarkSMSCodeUsed :exec
UPDATE sms_codes SET used = true WHERE id = $1;

-- name: DeleteSMSCodeByID :exec
DELETE FROM sms_codes WHERE id = $1;

-- name: DeleteExpiredSMSCodes :exec
DELETE FROM sms_codes WHERE expires_at < $1;

-- name: DeleteExpiredSMSCodesBatch :execrows
DELETE FROM sms_codes t WHERE t.ctid IN (
    SELECT s.ctid FROM sms_codes s WHERE s.expires_at < $1 LIMIT $2
);

-- name: GetLoginAttemptByPhone :one
SELECT * FROM login_attempts WHERE phone = $1;

-- name: UpsertLoginAttempt :exec
INSERT INTO login_attempts (phone, failures, first_failure_at, last_failure_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (phone) DO UPDATE SET
    failures = EXCLUDED.failures,
    first_failure_at = EXCLUDED.first_failure_at,
    last_failure_at = EXCLUDED.last_failure_at;

-- name: DeleteLoginAttemptByPhone :exec
DELETE FROM login_attempts WHERE phone = $1;

-- name: DeleteStaleLoginAttempts :exec
DELETE FROM login_attempts WHERE last_failure_at < $1;

-- name: DeleteStaleLoginAttemptsBatch :execrows
DELETE FROM login_attempts t WHERE t.ctid IN (
    SELECT a.ctid FROM login_attempts a WHERE a.last_failure_at < $1 LIMIT $2
);

-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, expires_at)
VALUES ($1, $2, $3) RETURNING *;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions WHERE token_hash = $1;

-- name: DeleteExpiredSessions :exec
DELETE FROM sessions WHERE expires_at < $1;

-- name: DeleteExpiredSessionsBatch :execrows
DELETE FROM sessions t WHERE t.ctid IN (
    SELECT s.ctid FROM sessions s WHERE s.expires_at < $1 LIMIT $2
);

-- name: GetSessionByTokenHash :one
SELECT s.id, s.token_hash, s.expires_at, s.created_at,
       u.id AS user_id, u.phone, u.role, u.name, u.surname, u.patronymic, u.email
FROM sessions s
JOIN users u ON s.user_id = u.id
WHERE s.token_hash = $1 AND s.expires_at > $2;

-- name: GetUserPhoneByID :one
SELECT phone FROM users WHERE id = $1;
