-- name: CreateReminder :one
INSERT INTO reminders (
    id, owner_id, target_type, operation_id, recurring_operation_id, lease_id, property_id,
    event_type, status, scheduled_at, message_title, message_body, created_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
)
RETURNING *;

-- name: GetReminderByIDAndOwner :one
SELECT * FROM reminders WHERE id = $1 AND owner_id = $2;

-- name: GetReminderByIDUnscoped :one
SELECT * FROM reminders WHERE id = $1;

-- name: ListRemindersByOwner :many
SELECT * FROM reminders
WHERE owner_id = sqlc.arg('owner_id')
  AND CASE
        WHEN sqlc.arg('filter_by_status')::boolean THEN status::text = sqlc.arg('status')::text
        ELSE true
      END
ORDER BY scheduled_at ASC
LIMIT sqlc.arg('limit') OFFSET sqlc.arg('offset');

-- name: ListRemindersByOperation :many
SELECT * FROM reminders
WHERE owner_id = $1 AND operation_id = $2 AND status != 'cancelled'
ORDER BY scheduled_at ASC
LIMIT $3 OFFSET $4;

-- name: ListRemindersByLease :many
SELECT * FROM reminders
WHERE owner_id = $1 AND lease_id = $2 AND status != 'cancelled'
ORDER BY scheduled_at ASC
LIMIT $3 OFFSET $4;

-- name: ListRemindersByRecurringOperation :many
SELECT * FROM reminders
WHERE owner_id = $1 AND recurring_operation_id = $2 AND status != 'cancelled'
ORDER BY scheduled_at ASC
LIMIT $3 OFFSET $4;

-- name: ListDueReminders :many
SELECT * FROM reminders
WHERE status = 'pending'
  AND scheduled_at <= $1
  AND (next_attempt_at IS NULL OR next_attempt_at <= $1)
ORDER BY scheduled_at ASC
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: ListStaleSendingReminders :many
SELECT * FROM reminders
WHERE status = 'sending'
  AND updated_at < $1
ORDER BY updated_at ASC
LIMIT $2
FOR UPDATE SKIP LOCKED;

-- name: UpdateReminderScheduledAt :execrows
UPDATE reminders
SET scheduled_at = $1
WHERE id = $2 AND owner_id = $3 AND status = 'pending';

-- name: MarkReminderSending :one
UPDATE reminders
SET status = 'sending'
WHERE id = $1 AND status = 'pending'
RETURNING *;

-- name: SaveOrReplaceOperationReminder :execrows
INSERT INTO reminders (
    id, owner_id, target_type, operation_id, recurring_operation_id, lease_id,
    property_id, event_type, status, scheduled_at, message_title, message_body,
    created_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
ON CONFLICT (owner_id, target_type, operation_id, recurring_operation_id, lease_id, event_type)
WHERE status IN ('pending', 'sending')
DO UPDATE SET
    scheduled_at = EXCLUDED.scheduled_at,
    message_title = EXCLUDED.message_title,
    message_body = EXCLUDED.message_body,
    status = EXCLUDED.status;

-- name: CancelByIDAndOwner :execrows
UPDATE reminders
SET status = 'cancelled'
WHERE id = $1 AND owner_id = $2 AND status IN ('pending', 'sending');

-- name: MarkSendingReminderSent :execrows
UPDATE reminders
SET status = 'sent', sent_at = $2
WHERE id = $1 AND status = 'sending';

-- name: MarkReminderSent :execrows
UPDATE reminders
SET status = 'sent', sent_at = $1
WHERE id = $2 AND status IN ('pending', 'sending');

-- name: MarkReminderFailed :execrows
UPDATE reminders
SET failed_attempts = failed_attempts + 1,
    next_attempt_at = sqlc.arg('next_attempt_at')::timestamptz,
    status = CASE WHEN sqlc.arg('mark_as_failed')::boolean THEN 'failed'::notification_status ELSE 'pending'::notification_status END
WHERE id = sqlc.arg('id')::uuid AND status IN ('pending', 'sending');

-- name: ResetReminderSending :execrows
UPDATE reminders
SET status = 'pending'
WHERE id = $1 AND status = 'sending';

-- name: MarkSendingReminderPending :execrows
UPDATE reminders
SET status = 'pending',
    next_attempt_at = sqlc.arg('next_attempt_at')::timestamptz,
    failed_attempts = failed_attempts + 1
WHERE id = sqlc.arg('id')::uuid AND status = 'sending';

-- name: CancelReminderByTarget :execrows
UPDATE reminders
SET status = 'cancelled'
WHERE owner_id = sqlc.arg('owner_id')
  AND target_type = sqlc.arg('target_type')
  AND (
      (target_type = 'operation' AND operation_id = sqlc.arg('target_id')::uuid) OR
      (target_type = 'recurring_operation' AND recurring_operation_id = sqlc.arg('target_id')::uuid) OR
      (target_type = 'lease' AND lease_id = sqlc.arg('target_id')::uuid)
  )
  AND event_type = sqlc.arg('event_type')
  AND status IN ('pending', 'sending');

-- name: CancelRemindersByRecurringOperationID :execrows
UPDATE reminders
SET status = 'cancelled'
WHERE owner_id = $1
  AND recurring_operation_id = $2
  AND status IN ('pending', 'sending');

-- name: HasReminderForLeaseEvent :one
SELECT EXISTS(SELECT 1 FROM reminders WHERE owner_id = $1 AND lease_id = $2 AND event_type = $3 AND status IN ('pending', 'sending', 'sent')) AS exists;

-- name: HasReminderForOperationEvent :one
SELECT EXISTS(
    SELECT 1 FROM reminders
    WHERE owner_id = $1 AND operation_id = $2 AND event_type = $3
      AND status IN ('pending', 'sending', 'sent')
) AS exists;

-- name: IsSMSReminderSent :one
SELECT EXISTS(SELECT 1 FROM sent_sms_reminders WHERE reminder_id = $1) AS exists;

-- name: CreateSentSMSReminder :execrows
INSERT INTO sent_sms_reminders (id, reminder_id, owner_id, phone, message, provider_response, sent_at)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (reminder_id) DO NOTHING;

-- name: UpdateSentSMSReminderProviderResponse :execrows
UPDATE sent_sms_reminders
SET provider_response = $1
WHERE reminder_id = $2;

-- name: IsEmailReminderSent :one
SELECT EXISTS (
    SELECT 1 FROM sent_email_reminders WHERE reminder_id = $1
);

-- name: SaveSentEmailReminder :execrows
INSERT INTO sent_email_reminders (
    id, reminder_id, owner_id, email, subject, plain_body, sent_at
) VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (reminder_id) DO NOTHING;

-- name: DeleteSentEmailReminder :exec
DELETE FROM sent_email_reminders WHERE reminder_id = $1;
