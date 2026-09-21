-- name: GetUserByID :one
SELECT id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted, email_verified_at, timezone FROM users WHERE id = $1;

-- name: GetUserByIDForUpdate :one
SELECT id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted, email_verified_at, timezone FROM users WHERE id = $1 FOR UPDATE;

-- name: GetUserByPhone :one
SELECT id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted, email_verified_at, timezone FROM users WHERE phone = $1;

-- name: GetUserByPhoneForUpdate :one
SELECT id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted, email_verified_at, timezone FROM users WHERE phone = $1 FOR UPDATE;

-- name: GetUserByEmail :one
SELECT id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted, email_verified_at, timezone FROM users WHERE LOWER(email) = LOWER($1::text);

-- name: GetUserByEmailForUpdate :one
SELECT id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted, email_verified_at, timezone FROM users WHERE LOWER(email) = LOWER($1::text) FOR UPDATE;

-- name: CreateUser :one
INSERT INTO users (id, phone, role, phone_encrypted, email, email_verified_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT DO NOTHING
RETURNING id, phone, role, name, surname, patronymic, email, email_verified_at, created_at, updated_at, phone_encrypted, timezone;

-- GetLatestLoginCodeByPhoneAndEmailAndPurpose reads the newest non-expired unused
-- login code for a (phone, email, purpose) tuple. It is served by the partial unique
-- index idx_login_codes_unique_unused (phone, COALESCE(email, empty-string), purpose)
-- WHERE used=false created by migration 000063 — the same index that backs
-- CreateLoginCode's ON CONFLICT upsert. The index serves both writes and reads; no
-- separate read index is needed (issue #224, EXPLAIN-verified: at production scale
-- the planner chooses an index scan over this index; email falls into Filter, not
-- Index Cond, because the index column is COALESCE(email, ...) and cannot match a raw
-- email=$2 equality, but the phone+purpose prefix narrows the scan efficiently).
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

-- ResetLoginAttempt writes the absolute attempt-window state, used when the
-- window is new or has expired (TTL reset) and the failure counter must be set
-- to an absolute value rather than incremented.
-- name: ResetLoginAttempt :exec
INSERT INTO login_attempts (id, phone, failures, first_failure_at, last_failure_at, user_id, phone_encrypted)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (phone) DO UPDATE SET
    failures = EXCLUDED.failures,
    first_failure_at = EXCLUDED.first_failure_at,
    last_failure_at = EXCLUDED.last_failure_at,
    user_id = EXCLUDED.user_id,
    phone_encrypted = EXCLUDED.phone_encrypted;

-- IncrementLoginAttempt atomically increments the failure counter on the
-- existing row, so concurrent upserts cannot lose an increment (issue #215).
-- failures carries the delta to add; first_failure_at is intentionally left
-- untouched on the conflict branch because the window is not being reset.
-- name: IncrementLoginAttempt :exec
INSERT INTO login_attempts (id, phone, failures, first_failure_at, last_failure_at, user_id, phone_encrypted)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (phone) DO UPDATE SET
    failures = login_attempts.failures + EXCLUDED.failures,
    last_failure_at = EXCLUDED.last_failure_at,
    user_id = EXCLUDED.user_id;

-- name: DeleteLoginAttemptByPhone :exec
DELETE FROM login_attempts WHERE phone = $1;

-- name: DeleteLoginAttemptsByUserID :exec
DELETE FROM login_attempts WHERE user_id = $1;

-- name: DeleteStaleLoginAttemptsBatch :execrows
DELETE FROM login_attempts t WHERE t.ctid IN (
    SELECT a.ctid FROM login_attempts a WHERE a.last_failure_at < $1 LIMIT $2
);

-- name: CreateSession :one
INSERT INTO sessions (id, user_id, token_hash, expires_at, last_used_at, rotated_at, last_ip, user_agent, device_type, browser, browser_major, os, city)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
RETURNING id, user_id, token_hash, expires_at, created_at, last_used_at;

-- TouchSession persists one request's activity: the sliding expiry, the
-- throttled last-seen stamp, and the client IP with its GeoIP city (null when
-- unresolvable — an existing city is never erased by an unknown one, the
-- application only writes cities it actually resolved).
-- name: TouchSession :exec
UPDATE sessions SET expires_at = sqlc.arg('expires_at'), last_used_at = sqlc.arg('last_used_at'), last_ip = sqlc.arg('last_ip'), city = sqlc.arg('city')
WHERE token_hash = sqlc.arg('token_hash')::text;

-- RotateSessionToken swaps the session token in place: the fresh hash replaces
-- the old one, which moves to previous_token_hash (accepted for the in-flight
-- grace window by GetSessionByTokenHash) and rotated_at restarts the renewal
-- window. The token_hash = $3 guard makes a concurrent rotation a no-op for
-- the loser, so exactly one swap wins per window.
-- name: RotateSessionToken :execrows
UPDATE sessions SET
    token_hash = sqlc.arg('new_token_hash')::text,
    previous_token_hash = sqlc.arg('old_token_hash')::text,
    rotated_at = sqlc.arg('rotated_at'),
    expires_at = sqlc.arg('expires_at'),
    last_used_at = sqlc.arg('last_used_at'),
    last_ip = sqlc.arg('last_ip'),
    city = sqlc.arg('city')
WHERE id = sqlc.arg('id') AND token_hash = sqlc.arg('old_token_hash')::text;

-- name: DeleteSessionByTokenHash :exec
DELETE FROM sessions WHERE token_hash = sqlc.arg('token_hash')::text OR previous_token_hash = sqlc.arg('token_hash')::text;

-- The previous_token_hash branch is the SQL copy of the currentness rule in
-- identity/application/sessions_service.go (isCurrentSession): within the
-- rotation grace window the caller may present the previous token, so its row
-- must survive the except-delete behind RevokeOthers. Change the two copies
-- together; pinned by TestSessionRepository_ExceptDeleteHonorsRotationGrace.
-- name: DeleteSessionsByUserIDExcept :execrows
DELETE FROM sessions WHERE user_id = $1 AND token_hash <> $2 AND (previous_token_hash IS NULL OR previous_token_hash <> $2);

-- name: DeleteExpiredSessionsBatch :execrows
DELETE FROM sessions t WHERE t.ctid IN (
    SELECT s.ctid FROM sessions s WHERE s.expires_at < $1 LIMIT $2
);

-- GetSessionByTokenHash resolves a live session by its token hash. The
-- previous_token_hash branch keeps in-flight requests working during the
-- rotation grace window: the old token is accepted until rotated_at falls
-- behind $2 (now - SessionRotationGrace).
-- name: GetSessionByTokenHash :one
SELECT s.id, s.token_hash, s.previous_token_hash, s.expires_at, s.created_at, s.last_used_at, s.rotated_at,
       s.last_ip, s.user_agent, s.device_type, s.browser, s.browser_major, s.os, s.city,
       u.id AS user_id, u.phone, u.role, u.name, u.surname, u.patronymic, u.email, u.email_verified_at, u.phone_encrypted, u.timezone
FROM sessions s
JOIN users u ON s.user_id = u.id
WHERE (s.token_hash = sqlc.arg('token_hash')::text OR (s.previous_token_hash = sqlc.arg('token_hash')::text AND s.rotated_at > sqlc.arg('grace_cutoff'))) AND s.expires_at > sqlc.arg('seen_after');

-- name: ListSessionsByUserID :many
-- The devices list shows live sessions only: expired rows survive up to the
-- cleaner retention (a week) after expires_at, and GetSessionByTokenHash
-- already refuses them — the list must not show them as active either.
SELECT id, token_hash, device_type, browser, browser_major, os, city, last_ip, last_used_at, created_at, expires_at
FROM sessions
WHERE user_id = sqlc.arg('user_id') AND expires_at > sqlc.arg('seen_after')
ORDER BY last_used_at DESC, id DESC;

-- name: GetSessionByID :one
SELECT id, user_id, token_hash, device_type, browser, browser_major, os, city, last_ip, last_used_at, created_at, expires_at
FROM sessions
WHERE id = $1;

-- name: DeleteSessionByIDForUser :execrows
DELETE FROM sessions WHERE id = $1 AND user_id = $2;

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
    timezone = $7,
    updated_at = now()
WHERE id = $1
RETURNING id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted, email_verified_at, timezone;

-- name: UpdateUserPhone :one
UPDATE users
SET phone = $2,
    phone_encrypted = $3,
    updated_at = now()
WHERE id = $1
RETURNING id, phone, role, name, surname, patronymic, email, created_at, updated_at, phone_encrypted, email_verified_at, timezone;

-- name: UpdateUserEmailVerified :one
UPDATE users
SET email = $2,
    email_verified_at = $3,
    updated_at = now()
WHERE id = $1
RETURNING id, phone, role, name, surname, patronymic, email, email_verified_at, created_at, updated_at, phone_encrypted, timezone;

-- name: UpdateUserEmailVerifiedAt :one
UPDATE users
SET email_verified_at = $2,
    updated_at = now()
WHERE id = $1
RETURNING id, phone, role, name, surname, patronymic, email, email_verified_at, created_at, updated_at, phone_encrypted, timezone;

-- name: InsertEmailChangeGrant :exec
INSERT INTO email_change_grants (id, user_id, email, token_hash, expires_at, created_at)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetEmailChangeGrantByUserIDForUpdate :one
SELECT id, user_id, email, token_hash, expires_at, created_at FROM email_change_grants WHERE user_id = $1 FOR UPDATE;

-- name: DeleteEmailChangeGrantByID :exec
DELETE FROM email_change_grants WHERE id = $1;

-- name: DeleteEmailChangeGrantsByUserID :exec
DELETE FROM email_change_grants WHERE user_id = $1;

-- name: ListUsersAdmin :many
SELECT u.*, us.status AS subscription_status
FROM users u
LEFT JOIN user_subscriptions us ON us.user_id = u.id
WHERE (sqlc.arg('phone')::text = '' OR u.phone = sqlc.arg('phone_enc')::text OR (u.phone = sqlc.arg('phone')::text AND u.phone_encrypted = false))
  AND (sqlc.arg('email')::text = '' OR LOWER(u.email) LIKE LOWER('%' || sqlc.arg('email') || '%'))
  AND (sqlc.arg('role')::text = '' OR u.role = sqlc.arg('role')::text)
  AND (sqlc.arg('subscription_status')::text = '' OR us.status = sqlc.arg('subscription_status')::text)
ORDER BY
  CASE WHEN sqlc.arg('sort')::text = 'createdAt' AND sqlc.arg('order')::text = 'asc' THEN u.created_at END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'createdAt' AND sqlc.arg('order')::text = 'desc' THEN u.created_at END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'updatedAt' AND sqlc.arg('order')::text = 'asc' THEN u.updated_at END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'updatedAt' AND sqlc.arg('order')::text = 'desc' THEN u.updated_at END DESC,
  CASE WHEN sqlc.arg('sort')::text = '' THEN u.created_at END DESC,
  u.id DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountUsersAdmin :one
SELECT COUNT(*)
FROM users u
LEFT JOIN user_subscriptions us ON us.user_id = u.id
WHERE (sqlc.arg('phone')::text = '' OR u.phone = sqlc.arg('phone_enc')::text OR (u.phone = sqlc.arg('phone')::text AND u.phone_encrypted = false))
  AND (sqlc.arg('email')::text = '' OR LOWER(u.email) LIKE LOWER('%' || sqlc.arg('email') || '%'))
  AND (sqlc.arg('role')::text = '' OR u.role = sqlc.arg('role')::text)
  AND (sqlc.arg('subscription_status')::text = '' OR us.status = sqlc.arg('subscription_status')::text);

-- GetUserByIDAdmin is implemented by the existing GetUserByID query (no owner filter).

-- name: CountUsersTotalAdmin :one
SELECT COUNT(*) FROM users;

-- name: CountNewUsersLast30dAdmin :one
SELECT COUNT(*) FROM users
WHERE created_at >= now() - interval '30 days';

-- name: ListRecentUsersAdmin :many
SELECT id, phone, phone_encrypted, name, surname, created_at
FROM users
ORDER BY created_at DESC, id DESC
LIMIT 5;
