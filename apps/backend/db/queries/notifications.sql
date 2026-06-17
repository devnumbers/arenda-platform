-- name: CreateReminder :one
INSERT INTO reminders (
    id, owner_id, target_type, operation_id, recurring_operation_id, lease_id, property_id,
    event_type, status, scheduled_at, event_date, message_title, message_body, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
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

-- name: UpdateReminder :execrows
UPDATE reminders
SET owner_id = $2,
    target_type = $3,
    operation_id = $4,
    recurring_operation_id = $5,
    lease_id = $6,
    property_id = $7,
    event_type = $8,
    status = $9,
    scheduled_at = $10,
    event_date = $11,
    sent_at = $12,
    failed_attempts = $13,
    next_attempt_at = $14,
    message_title = $15,
    message_body = $16,
    updated_at = $17
WHERE id = $1;

-- name: MarkReminderSent :execrows
UPDATE reminders
SET status = 'sent', sent_at = $2, updated_at = NOW()
WHERE id = $1 AND status = 'pending';

-- name: MarkReminderFailed :execrows
UPDATE reminders
SET failed_attempts = failed_attempts + 1,
    next_attempt_at = $2,
    status = CASE WHEN sqlc.arg('mark_as_failed')::boolean THEN 'failed' ELSE status END,
    updated_at = NOW()
WHERE id = $1 AND status = 'pending';

-- name: CancelReminderByTarget :execrows
UPDATE reminders
SET status = 'cancelled', updated_at = NOW()
WHERE owner_id = sqlc.arg('owner_id')
  AND target_type = sqlc.arg('target_type')
  AND (
      (target_type = 'operation' AND operation_id = sqlc.arg('target_id')::uuid) OR
      (target_type = 'recurring_operation' AND recurring_operation_id = sqlc.arg('target_id')::uuid) OR
      (target_type = 'lease' AND lease_id = sqlc.arg('target_id')::uuid)
  )
  AND event_type = sqlc.arg('event_type')
  AND status = 'pending';

-- name: CancelRemindersByRecurringOperationID :execrows
UPDATE reminders
SET status = 'cancelled', updated_at = NOW()
WHERE owner_id = $1
  AND recurring_operation_id = $2
  AND status = 'pending';

-- name: CreateSentSMSReminder :one
INSERT INTO sent_sms_reminders (id, reminder_id, owner_id, phone, message, provider_response, sent_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;
