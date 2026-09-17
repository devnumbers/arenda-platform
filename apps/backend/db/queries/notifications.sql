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
