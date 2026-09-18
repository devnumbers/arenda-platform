-- Хранимая лента уведомлений (карта #734, тикет #739; модель — решение
-- #737). One event = one row per recipient (fan-out); the row carries the
-- text snapshot, the payload links and the personal flags.

-- name: InsertNotification :execrows
-- Publishes one recipient's feed row. The unique (user_id, dedup_key) index
-- makes the dedup invariant durable: a repeat publication with the same key
-- inserts nothing (0 rows), it is a no-op rather than an error.
INSERT INTO notifications (id, user_id, category, event_type, title, body, context_label, payload, dedup_key)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
ON CONFLICT (user_id, dedup_key) DO NOTHING;

-- name: ListNotifications :many
-- The user's feed page, newest first, deleted rows never appear. The walk
-- resumes strictly after the (created_at, id) the previous page ended on
-- (канон #597), so rows created between loads never duplicate or drop; both
-- cursor args travel together, NULL reads from the beginning. unread_only
-- filters the page to unread rows (the partial index
-- idx_notifications_user_unread serves the filtered walk); page_limit 0 = no
-- limit.
SELECT *
FROM notifications
WHERE user_id = $1
  AND deleted_at IS NULL
  AND (sqlc.arg('unread_only')::bool = false OR read_at IS NULL)
  AND (sqlc.narg('after_created_at')::timestamptz IS NULL
       OR (created_at, id) < (sqlc.narg('after_created_at'),
                              sqlc.narg('after_id')::uuid))
ORDER BY created_at DESC, id DESC
LIMIT CASE WHEN sqlc.arg('page_limit')::bigint = 0 THEN NULL::bigint
           ELSE sqlc.arg('page_limit')::bigint END;

-- name: CountUnreadNotifications :one
-- The unread counter (решение #737): unread, not-deleted rows of the user;
-- clicks, page opens and «Прочитать все» drive it down, push/email do not
-- touch it.
SELECT COUNT(*)
FROM notifications
WHERE user_id = $1
  AND read_at IS NULL
  AND deleted_at IS NULL;

-- name: MarkNotificationRead :execrows
-- Marks one row read (a click, the notification page). Idempotent: an
-- already-read or deleted row matches nothing (0 rows).
UPDATE notifications
SET read_at = now()
WHERE id = $1
  AND user_id = $2
  AND read_at IS NULL
  AND deleted_at IS NULL;

-- name: MarkAllNotificationsRead :execrows
-- «Прочитать все» (решение #737): every unread not-deleted row of the user
-- in one statement; the result is the number of rows that flipped.
UPDATE notifications
SET read_at = now()
WHERE user_id = $1
  AND read_at IS NULL
  AND deleted_at IS NULL;

-- name: GetNotification :one
-- One feed row by id for the delivery jobs (#740): a job reloads the
-- committed row (recipient, texts, payload) instead of carrying content in
-- its args. A soft-deleted row still resolves — deletion hides the row from
-- the feed, it does not retract an in-flight delivery.
SELECT * FROM notifications WHERE id = $1;

-- name: DeleteNotification :execrows
-- Soft-deletes one feed row («удалить», решение #737): the row stays with
-- its deleted_at, the feed and the unread counter stop seeing it. Rows
-- matched = 0 means already deleted, read-status aside — the caller maps
-- that to not-found for the reader.
UPDATE notifications
SET deleted_at = now()
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

-- name: DeleteAllNotifications :execrows
-- «Удалить все» (решение #737): soft-deletes every feed row of the user in
-- one statement; the result is the number of rows hidden.
UPDATE notifications
SET deleted_at = now()
WHERE user_id = $1 AND deleted_at IS NULL;

-- name: GetEmailPreferences :one
-- The account-level email matrix (решение #738, ADR 0056): four
-- configurable categories. A missing row is the all-on default — the caller
-- falls back without inserting.
SELECT rental, payments_operations, tasks, shared_access
FROM notification_email_preferences
WHERE user_id = $1;

-- name: UpsertEmailPreferences :execrows
-- PUT /notification-preferences is a full replacement of the four flags
-- (канон #738).
INSERT INTO notification_email_preferences (user_id, rental, payments_operations, tasks, shared_access)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (user_id) DO UPDATE SET
    rental              = EXCLUDED.rental,
    payments_operations = EXCLUDED.payments_operations,
    tasks               = EXCLUDED.tasks,
    shared_access       = EXCLUDED.shared_access;

-- name: GetRentalActionState :one
-- The live rental facts behind the «Продлить»/«Завершить» buttons
-- (решение #737: действия вычисляются при чтении): whether the rental still
-- awaits action (not completed, planned end already passed — the
-- needs_attention state, ADR 0053) and which property it belongs to, so the
-- reader's rights resolve against the live rental, not the payload snapshot.
-- No row — the rental is gone.
SELECT r.completed_date, r.planned_end_date, r.start_date, r.property_id, p.owner_id
FROM rentals r
JOIN properties p ON p.id = r.property_id
WHERE r.id = $1;

-- name: GetOperationOpenState :one
-- Whether the notification's operation (Операция — вхождение Payments,
-- not the rule) still awaits payment: it exists and its status is planned.
-- Paid or cancelled — the state has moved on, the «открыть платёж» button
-- goes (решение #737).
SELECT EXISTS (SELECT 1 FROM operations WHERE id = $1 AND status = 'planned') AS open;

-- name: GetTaskOpenState :one
-- Whether the notification's task still awaits action: it exists and is not
-- completed. A completed or deleted task leaves the notification without
-- its button (решение #737).
SELECT EXISTS (SELECT 1 FROM tasks WHERE id = $1 AND completed_date IS NULL) AS open;

-- name: GetPropertyAccessFor :one
-- The reader's live role on the property (ADR 0028): 'owner' for the owner,
-- the active membership's role for a shared one, '' for a stranger. No row
-- — the property is gone. Exists = the role is non-empty; manageable = the
-- owner or a full_access member (the «Продлить»/«Завершить» gate).
SELECT COALESCE(
           CASE
               WHEN p.owner_id = $2 THEN 'owner'
               ELSE (SELECT m.role
                     FROM property_members m
                     WHERE m.property_id = p.id AND m.user_id = $2 AND m.status = 'active'
                     LIMIT 1)
           END::text,
           '') AS access_role
FROM properties p
WHERE p.id = $1;
