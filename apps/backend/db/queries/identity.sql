-- name: GetUserByID :one
SELECT id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted, email_verified_at FROM users WHERE id = $1;

-- name: GetUserByIDForUpdate :one
SELECT id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted, email_verified_at FROM users WHERE id = $1 FOR UPDATE;

-- name: GetUserByPhone :one
SELECT id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted, email_verified_at FROM users WHERE phone = $1;

-- name: GetUserByPhoneForUpdate :one
SELECT id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted, email_verified_at FROM users WHERE phone = $1 FOR UPDATE;

-- name: CreateUser :one
INSERT INTO users (id, phone, role, phone_encrypted, email, email_verified_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (phone) DO NOTHING
RETURNING id, phone, role, name, surname, patronymic, email, email_verified_at, created_at, updated_at, phone_encrypted;

-- name: GetLatestLoginCodeByPhoneAndEmailAndPurpose :one
SELECT id, user_id, phone, email, code_hash, expires_at, used, created_at, purpose, phone_encrypted FROM login_codes
WHERE phone = $1 AND email = $2 AND purpose = $3 AND used = false AND expires_at > $4
ORDER BY created_at DESC
LIMIT 1
FOR UPDATE;

-- name: CreateLoginCode :exec
INSERT INTO login_codes (id, phone, email, code_hash, expires_at, user_id, purpose, phone_encrypted)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (phone, COALESCE(email, ''), purpose) WHERE used = false
DO UPDATE SET
    id = EXCLUDED.id,
    code_hash = EXCLUDED.code_hash,
    expires_at = EXCLUDED.expires_at,
    user_id = EXCLUDED.user_id,
    phone_encrypted = EXCLUDED.phone_encrypted,
    created_at = now();

-- name: DeleteExpiredLoginCodesByPhoneAndEmail :exec
DELETE FROM login_codes
WHERE phone = $1 AND email = $2 AND purpose = $3 AND used = false AND expires_at < $4;

-- name: DeleteUnusedLoginCodesByPhoneAndEmail :exec
DELETE FROM login_codes
WHERE phone = $1 AND email = $2 AND purpose = $3 AND used = false;

-- name: MarkLoginCodeUsed :exec
UPDATE login_codes SET used = true WHERE id = $1;

-- name: DeleteLoginCodeByID :exec
DELETE FROM login_codes WHERE id = $1;

-- name: DeleteLoginCodesByUserID :exec
DELETE FROM login_codes WHERE user_id = $1;

-- name: DeleteExpiredLoginCodesBatch :execrows
DELETE FROM login_codes t WHERE t.ctid IN (
    SELECT s.ctid FROM login_codes s WHERE s.expires_at < $1 LIMIT $2
);

-- name: GetLoginAttemptByPhone :one
SELECT id, phone, failures, first_failure_at, last_failure_at, user_id, phone_encrypted FROM login_attempts WHERE phone = $1;

-- name: GetLoginAttemptByPhoneForUpdate :one
SELECT id, phone, failures, first_failure_at, last_failure_at, user_id, phone_encrypted FROM login_attempts WHERE phone = $1 FOR UPDATE;

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

-- name: DeleteExpiredSessionsBatch :execrows
DELETE FROM sessions t WHERE t.ctid IN (
    SELECT s.ctid FROM sessions s WHERE s.expires_at < $1 LIMIT $2
);

-- name: GetSessionByTokenHash :one
SELECT s.id, s.token_hash, s.expires_at, s.created_at, s.last_used_at,
       u.id AS user_id, u.phone, u.role, u.name, u.surname, u.patronymic, u.email, u.email_verified_at, u.phone_encrypted
FROM sessions s
JOIN users u ON s.user_id = u.id
WHERE s.token_hash = $1 AND s.expires_at > $2;

-- name: GetVerifiedEmailByUserID :one
SELECT email
FROM users
WHERE id = $1 AND email_verified_at IS NOT NULL;

-- name: UpdateUser :one
UPDATE users
SET name = $2,
    surname = $3,
    patronymic = $4,
    email = $5,
    email_verified_at = $6,
    updated_at = now()
WHERE id = $1
RETURNING id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted, email_verified_at;

-- name: UpdateUserPhone :one
UPDATE users
SET phone = $2,
    phone_encrypted = $3,
    updated_at = now()
WHERE id = $1
RETURNING id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted, email_verified_at;

-- name: UpdateUserEmailVerified :one
UPDATE users
SET email = $2,
    email_verified_at = $3,
    updated_at = now()
WHERE id = $1
RETURNING id, phone, role, name, surname, patronymic, email, email_verified_at, created_at, updated_at, phone_encrypted;

-- name: ListUsersAdmin :many
SELECT u.*, us.status AS subscription_status
FROM users u
LEFT JOIN user_subscriptions us ON us.user_id = u.id
WHERE (sqlc.arg('phone')::text = '' OR u.phone = sqlc.arg('phone')::text)
  AND (sqlc.arg('email')::text = '' OR LOWER(u.email) LIKE LOWER('%' || sqlc.arg('email') || '%'))
  AND (sqlc.arg('role')::text = '' OR u.role = sqlc.arg('role')::text)
  AND (sqlc.arg('subscription_status')::text = '' OR us.status = sqlc.arg('subscription_status')::text)
ORDER BY u.created_at DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountUsersAdmin :one
SELECT COUNT(*)
FROM users u
LEFT JOIN user_subscriptions us ON us.user_id = u.id
WHERE (sqlc.arg('phone')::text = '' OR u.phone = sqlc.arg('phone')::text)
  AND (sqlc.arg('email')::text = '' OR LOWER(u.email) LIKE LOWER('%' || sqlc.arg('email') || '%'))
  AND (sqlc.arg('role')::text = '' OR u.role = sqlc.arg('role')::text)
  AND (sqlc.arg('subscription_status')::text = '' OR us.status = sqlc.arg('subscription_status')::text);

-- GetUserByIDAdmin is implemented by the existing GetUserByID query (no owner filter).
