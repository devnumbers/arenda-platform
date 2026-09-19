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

-- name: GetNotificationForUser :one
-- The feed page's single row (GET /notifications/{id}, #743): scoped to the
-- reader and hidden once deleted — a foreign or deleted row does not exist
-- for them. The delivery jobs keep using the unscoped GetNotification.
SELECT * FROM notifications
WHERE id = $1 AND user_id = $2 AND deleted_at IS NULL;

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

-- name: GetPaymentOpenState :one
-- Whether the notification's own payment occurrence (решение #737: payment
-- = правило + дата операции) still awaits payment: a planned operation of
-- the rule dated that day exists. The date pins the check to the notified
-- occurrence — the rule's other occurrences say nothing about it. Paid or
-- cancelled — the state has moved on; the rule deleted — its occurrences
-- lose the payment link and the «открыть платёж» button goes with the dead
-- page (решение #737).
SELECT EXISTS (
    SELECT 1 FROM operations
    WHERE payment_id = $1 AND date = $2 AND status = 'planned'
) AS open;

-- name: GetTaskOpenState :one
-- Whether the notification's task still awaits action: it exists and is not
-- completed. A completed or deleted task leaves the notification without
-- its button (решение #737).
SELECT EXISTS (SELECT 1 FROM tasks WHERE id = $1 AND completed_date IS NULL) AS open;

-- name: ListRentalCompletedScanZones :many
-- The rental-completed scan's sweep targets (карта #734, #748; ADR 0048
-- p.3): the distinct owner timezones having unfinished rentals with a
-- planned end on non-archived properties — the only rentals the scan can
-- fire for (the ticks' status canon: an archived property's rentals
-- mutations are rejected, the action buttons would be dead ends).
-- Stateless — every run re-lists, no per-zone state is kept.
SELECT DISTINCT u.timezone
FROM rentals r
JOIN properties p ON p.id = r.property_id
JOIN users u ON u.id = p.owner_id
WHERE r.completed_date IS NULL
  AND r.planned_end_date IS NOT NULL
  AND p.status IN ('active', 'maintenance')
ORDER BY u.timezone;

-- name: ListRentalCompletedTargets :many
-- One zone's needs_attention rentals as of the zone's today (решение #737,
-- тип №1: the day after the planned end): not completed, planned end
-- strictly before today, non-archived property. The property snapshot
-- (name, address) travels for the publication cards (решение владельца
-- 19.09.2026, #745).
SELECT r.id AS rental_id,
       r.planned_end_date,
       p.id AS property_id,
       p.name AS property_name,
       p.address AS property_address,
       p.owner_id
FROM rentals r
JOIN properties p ON p.id = r.property_id
JOIN users u ON u.id = p.owner_id
WHERE u.timezone = $1
  AND r.completed_date IS NULL
  AND r.planned_end_date IS NOT NULL
  AND r.planned_end_date < $2::date
  AND p.status IN ('active', 'maintenance')
ORDER BY r.id;

-- name: ListPropertyActiveRecipients :many
-- The property's active members' user ids — the object events' recipients
-- besides the owner (решение #737: активные участники, «Просмотр»
-- включительно; a suspended membership is not an active participant).
SELECT user_id
FROM property_members
WHERE property_id = $1 AND status = 'active'
ORDER BY user_id;

-- name: ListPaymentScanZones :many
-- The payments scan's sweep targets (карта #734, #749; ADR 0048 p.3): the
-- distinct owner timezones having planned payment-rule operations on
-- non-archived properties — the only operations the scan can fire for (the
-- ticks' status canon, same as the rental scan; the join to payments also
-- keeps the manual facts and the deleted rules' orphans out). Stateless —
-- every run re-lists, no per-zone state is kept.
SELECT DISTINCT u.timezone
FROM operations o
JOIN payments pay ON pay.id = o.payment_id
JOIN properties p ON p.id = o.property_id
JOIN users u ON u.id = o.owner_id
WHERE o.status = 'planned'
  AND p.status IN ('active', 'maintenance')
ORDER BY u.timezone;

-- name: ListPaymentDueTargets :many
-- One zone's due-day operations as of the zone's today (решение #737, тип
-- №2: в день срока): planned, dated exactly today, on rules without the
-- auto-pay mode — an auto-pay rule's due occurrence is extinguished by the
-- tick the same day (ADR 0049) and never asks to be paid; if the auto
-- charge did not happen, the operation becomes overdue and the overdue leg
-- speaks. The rule's title travels (not the operation's snapshot): the
-- notification's link lands on the payment's page, the copy names what the
-- reader sees there.
SELECT pay.id AS payment_id,
       o.date,
       pay.title,
       o.amount_kopecks,
       p.id AS property_id,
       p.name AS property_name,
       p.address AS property_address,
       o.owner_id
FROM operations o
JOIN payments pay ON pay.id = o.payment_id
JOIN properties p ON p.id = o.property_id
JOIN users u ON u.id = o.owner_id
WHERE u.timezone = $1
  AND o.date = $2::date
  AND o.status = 'planned'
  AND pay.auto_pay = false
  AND p.status IN ('active', 'maintenance')
ORDER BY o.id;

-- name: ListPaymentOverdueTargets :many
-- One zone's overdue operations as of the zone's today (решение #737, тип
-- №3: 1-й день просрочки): planned, dated strictly before today — auto-pay
-- rules included, the tick never backdates an auto charge (ADR 0049). The
-- dedup key (rule, operation date) keeps a long-unpaid operation single —
-- the sweep lists it daily, the publication inserts nothing.
SELECT pay.id AS payment_id,
       o.date,
       pay.title,
       o.amount_kopecks,
       p.id AS property_id,
       p.name AS property_name,
       p.address AS property_address,
       o.owner_id
FROM operations o
JOIN payments pay ON pay.id = o.payment_id
JOIN properties p ON p.id = o.property_id
JOIN users u ON u.id = o.owner_id
WHERE u.timezone = $1
  AND o.date < $2::date
  AND o.status = 'planned'
  AND p.status IN ('active', 'maintenance')
ORDER BY o.id;

