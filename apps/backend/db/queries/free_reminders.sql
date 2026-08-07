-- name: CreateFreeReminder :one
INSERT INTO free_reminders (
    id, owner_id, property_id, title, trigger_at, periodicity, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
)
RETURNING *;

-- name: GetFreeReminderByIDAndOwner :one
SELECT * FROM free_reminders
WHERE id = $1 AND owner_id = $2;

-- name: GetFreeReminderByIDUnscoped :one
SELECT * FROM free_reminders
WHERE id = $1;

-- name: UpdateFreeReminder :one
UPDATE free_reminders
SET title = $3,
    trigger_at = $4,
    periodicity = $5,
    updated_at = $6
WHERE id = $1 AND owner_id = $2
RETURNING *;

-- name: DeleteFreeReminderByIDAndOwner :execrows
DELETE FROM free_reminders
WHERE id = $1 AND owner_id = $2;

-- name: ListFreeRemindersByOwner :many
SELECT * FROM free_reminders
WHERE owner_id = $1
ORDER BY trigger_at ASC
LIMIT $2 OFFSET $3;

-- name: ListFreeRemindersByProperty :many
SELECT * FROM free_reminders
WHERE owner_id = $1 AND property_id = $2
ORDER BY trigger_at ASC
LIMIT $3;

-- name: ListAllFreeRemindersByOwner :many
SELECT fr.*, p.name AS property_name
FROM free_reminders fr
LEFT JOIN properties p ON p.id = fr.property_id
WHERE fr.owner_id = sqlc.arg('owner_id')
ORDER BY fr.trigger_at ASC;
