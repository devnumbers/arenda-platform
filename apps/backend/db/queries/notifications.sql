-- name: CreateReminder :one
INSERT INTO reminders (
    id, owner_id, target_type, operation_id, recurring_operation_id, lease_id, property_id,
    event_type, status, scheduled_at, message_title, message_body, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14
)
RETURNING *;

-- name: GetReminderByID :one
SELECT * FROM reminders WHERE id = $1;

-- name: ListRemindersByOwner :many
SELECT * FROM reminders
WHERE owner_id = sqlc.arg('owner_id')
  AND (NOT sqlc.arg('filter_by_status')::boolean OR status = sqlc.arg('status'))
ORDER BY scheduled_at ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: ListDueReminders :many
SELECT * FROM reminders
WHERE status = 'pending'
  AND scheduled_at <= $1
  AND (next_attempt_at IS NULL OR next_attempt_at <= $1)
ORDER BY scheduled_at ASC
LIMIT $2;

-- name: MarkReminderSent :execrows
UPDATE reminders
SET status = 'sent', sent_at = $2, updated_at = NOW()
WHERE id = $1 AND status = 'pending';

-- name: MarkReminderFailed :execrows
UPDATE reminders
SET failed_attempts = failed_attempts + 1,
    next_attempt_at = $2,
    status = CASE WHEN $3::boolean THEN 'failed' ELSE status END,
    updated_at = NOW()
WHERE id = $1 AND status = 'pending';

-- name: CancelReminderByTarget :execrows
UPDATE reminders
SET status = 'cancelled', updated_at = NOW()
WHERE owner_id = $1
  AND target_type = $2
  AND (
      (target_type = 'operation' AND operation_id = $3) OR
      (target_type = 'recurring_operation' AND recurring_operation_id = $3) OR
      (target_type = 'lease' AND lease_id = $3)
  )
  AND event_type = $4
  AND status = 'pending';

-- name: CancelRemindersByOperationIDs :execrows
UPDATE reminders
SET status = 'cancelled', updated_at = NOW()
WHERE owner_id = $1
  AND operation_id = ANY($2::uuid[])
  AND status = 'pending';

-- name: CreateSentSMSReminder :one
INSERT INTO sent_sms_reminders (id, reminder_id, owner_id, phone, message, provider_response, sent_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetUserPhoneByID :one
SELECT phone FROM users WHERE id = $1;
