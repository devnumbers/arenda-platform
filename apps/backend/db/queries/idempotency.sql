-- Идемпотентные ключи creations (тикет Т3 карты #1112): бронь ключа до
-- исполнения хендлера закрывает гонку двух параллельных POST с одним
-- ключом — второй получает сохранённый ответ или 409, но не второй 201.

-- name: ReserveIdempotencyKey :one
INSERT INTO idempotency_keys (owner_id, key, endpoint, request_hash)
VALUES ($1, $2, $3, $4)
ON CONFLICT (owner_id, key) DO NOTHING
RETURNING owner_id, key, endpoint, request_hash, content_type, status_code, response, created_at;

-- name: GetIdempotencyKey :one
SELECT owner_id, key, endpoint, request_hash, content_type, status_code, response, created_at
FROM idempotency_keys
WHERE owner_id = $1 AND key = $2;

-- name: CompleteIdempotencyKey :exec
UPDATE idempotency_keys
SET content_type = $3, status_code = $4, response = $5
WHERE owner_id = $1 AND key = $2 AND status_code IS NULL;

-- name: CleanupExpiredIdempotencyKeys :execrows
DELETE FROM idempotency_keys
WHERE created_at < now() - interval '24 hours';
