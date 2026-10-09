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
-- The account-level email matrix (решение #738, ADR 0058): four
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
       p.type AS property_type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/v1/properties/' || p.id::text || '/photo'
                     END, '')::text AS property_photo,
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

-- name: ListRentalScheduledCompletedTargets :many
-- The rental scan's booking list of the completed boundary (issue #777):
-- the unfinished rentals whose boundary — 00:00 of the day after the
-- planned end read in the owner's timezone — falls in the window (from,
-- until]. The wall-clock midnight of the next calendar date is the
-- boundary, a DST day rolls it with the wall clock (the payments' overdue
-- convention); non-archived property only (the ticks' canon).
SELECT r.id AS rental_id,
       r.planned_end_date,
       CAST(((r.planned_end_date + 1)::timestamp AT TIME ZONE u.timezone) AS timestamptz) AS fire_at
FROM rentals r
JOIN properties p ON p.id = r.property_id
JOIN users u ON u.id = p.owner_id
WHERE r.completed_date IS NULL
  AND r.planned_end_date IS NOT NULL
  AND p.status IN ('active', 'maintenance')
  AND ((r.planned_end_date + 1)::timestamp AT TIME ZONE u.timezone) > $1::timestamptz
  AND ((r.planned_end_date + 1)::timestamp AT TIME ZONE u.timezone) <= $2::timestamptz
ORDER BY r.id;

-- name: GetScheduledCompletedRental :one
-- The completed boundary job's delivery-time resolution (issue #777): the
-- rental as it stands at its boundary midnight. The needs_attention
-- conditions are re-checked as of the wake-up instant ($3): a completed
-- rental, an extended one (the planned end moved off the booked date — its
-- new boundary books its own job), an archived property — and a rental
-- whose planned end is no longer strictly before the zone's today (the job
-- woke before the boundary) — answers no row, the job finishes without
-- publishing.
SELECT r.id AS rental_id,
       r.planned_end_date,
       p.id AS property_id,
       p.name AS property_name,
       p.address AS property_address,
       p.type AS property_type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/v1/properties/' || p.id::text || '/photo'
                     END, '')::text AS property_photo,
       p.owner_id
FROM rentals r
JOIN properties p ON p.id = r.property_id
JOIN users u ON u.id = p.owner_id
WHERE r.id = $1::uuid
  AND r.planned_end_date = $2::date
  AND r.completed_date IS NULL
  AND p.status IN ('active', 'maintenance')
  AND r.planned_end_date < ($3::timestamptz AT TIME ZONE u.timezone)::date;

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
-- reader sees there. The instant gate (#1168, решение владельца 06.10 по
-- гриллингу #1167): the leg's boundary is the wall clock 10:00 of the
-- operation date in the owner's timezone — the hourly backstop sweep must
-- not publish ahead of it, the same instant predicate the booking leg and
-- the tasks scan use.
SELECT pay.id AS payment_id,
       o.date,
       pay.title,
       o.amount_kopecks,
       p.id AS property_id,
       p.name AS property_name,
       p.address AS property_address,
       p.type AS property_type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/v1/properties/' || p.id::text || '/photo'
                     END, '')::text AS property_photo,
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
  AND ((o.date::timestamp + time '10:00') AT TIME ZONE u.timezone) <= $3::timestamptz
ORDER BY o.id;

-- name: ListPaymentOverdueTargets :many
-- One zone's overdue operations as of the zone's today (решение #737, тип
-- №3: 1-й день просрочки): planned, dated strictly before today — auto-pay
-- rules included, the tick never backdates an auto charge (ADR 0049). The
-- dedup key (rule, operation date) keeps a long-unpaid operation single —
-- the sweep lists it daily, the publication inserts nothing. The instant
-- gate (#1168): the leg's boundary is the wall clock 22:00 of the day after
-- the operation date in the owner's timezone — yesterday's occurrence waits
-- for its 22:00 even though the date says overdue since the zone's midnight;
-- a long-unpaid operation's boundary is long past and the gate keeps it
-- sweeping (the persistent leg's semantics).
SELECT pay.id AS payment_id,
       o.date,
       pay.title,
       o.amount_kopecks,
       p.id AS property_id,
       p.name AS property_name,
       p.address AS property_address,
       p.type AS property_type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/v1/properties/' || p.id::text || '/photo'
                     END, '')::text AS property_photo,
       o.owner_id
FROM operations o
JOIN payments pay ON pay.id = o.payment_id
JOIN properties p ON p.id = o.property_id
JOIN users u ON u.id = o.owner_id
WHERE u.timezone = $1
  AND o.date < $2::date
  AND o.status = 'planned'
  AND p.status IN ('active', 'maintenance')
  AND (((o.date + 1)::timestamp + time '22:00') AT TIME ZONE u.timezone) <= $3::timestamptz
ORDER BY o.id;

-- name: ListPaymentScheduledDueTargets :many
-- The payments scan's booking list of the due leg (issue #776): the planned
-- operations whose due boundary — the wall clock 10:00 of the operation
-- date read in the owner's timezone (#1168, решение по гриллингу #1167:
-- «Оплатите платёж» больше не будит в полночь) — falls in the window
-- (from, until]. Auto-pay rules
-- excluded like the sweep's due leg (an auto-pay rule's due occurrence is
-- extinguished by the tick the same day, ADR 0049); manual facts and
-- cancelled tombstones stay out (status='planned' + the join to payments);
-- non-archived property only (the ticks' canon).
SELECT pay.id AS payment_id,
       o.date,
       CAST((o.date::timestamp + time '10:00') AT TIME ZONE u.timezone AS timestamptz) AS fire_at
FROM operations o
JOIN payments pay ON pay.id = o.payment_id
JOIN properties p ON p.id = o.property_id
JOIN users u ON u.id = o.owner_id
WHERE o.status = 'planned'
  AND pay.auto_pay = false
  AND p.status IN ('active', 'maintenance')
  AND (o.date::timestamp AT TIME ZONE u.timezone) > $1::timestamptz
  AND (o.date::timestamp AT TIME ZONE u.timezone) <= $2::timestamptz
ORDER BY o.id;

-- name: ListPaymentScheduledOverdueTargets :many
-- The payments scan's booking list of the overdue leg (issue #776): the
-- planned operations whose overdue boundary — the wall clock 22:00 of the
-- day after the operation date read in the owner's timezone (#1168, по
-- гриллингу #1167: просрочка приходит вечером, не в полночь) — falls in the
-- window (from, until]. Auto-pay rules included (the tick never backdates an auto
-- charge, ADR 0049); the wall clock 22:00 is
-- the boundary, a DST day rolls it with the wall clock.
SELECT pay.id AS payment_id,
       o.date,
       CAST(((o.date + 1)::timestamp + time '22:00') AT TIME ZONE u.timezone AS timestamptz) AS fire_at
FROM operations o
JOIN payments pay ON pay.id = o.payment_id
JOIN properties p ON p.id = o.property_id
JOIN users u ON u.id = o.owner_id
WHERE o.status = 'planned'
  AND p.status IN ('active', 'maintenance')
  AND ((o.date + 1)::timestamp AT TIME ZONE u.timezone) > $1::timestamptz
  AND ((o.date + 1)::timestamp AT TIME ZONE u.timezone) <= $2::timestamptz
ORDER BY o.id;

-- name: GetScheduledDuePayment :one
-- The due boundary job's delivery-time resolution (issue #776): the
-- operation as it stands at its due boundary — the wall clock 10:00 of the
-- operation date in the owner's timezone (#1168). The leg's conditions are
-- re-checked as of the wake-up instant ($3): a paid, cancelled, auto-pay
-- operation, a deleted rule's orphan, an archived property — and an
-- operation whose date is no longer the zone's today (the job woke after
-- the day had rolled over) — answers no row, the job finishes without
-- publishing. The date-against-today predicate holds at any hour of the
-- boundary day — the wake-up instant itself needs no gate, the job books
-- at its own 10:00.
SELECT pay.id AS payment_id,
       o.date,
       pay.title,
       o.amount_kopecks,
       p.id AS property_id,
       p.name AS property_name,
       p.address AS property_address,
       p.type AS property_type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/v1/properties/' || p.id::text || '/photo'
                     END, '')::text AS property_photo,
       o.owner_id
FROM operations o
JOIN payments pay ON pay.id = o.payment_id
JOIN properties p ON p.id = o.property_id
JOIN users u ON u.id = o.owner_id
WHERE pay.id = $1::uuid
  AND o.date = $2::date
  AND o.status = 'planned'
  AND pay.auto_pay = false
  AND p.status IN ('active', 'maintenance')
  AND o.date = ($3::timestamptz AT TIME ZONE u.timezone)::date;

-- name: GetScheduledOverduePayment :one
-- The overdue boundary job's delivery-time resolution (issue #776): the
-- operation as it stands at its overdue boundary — the wall clock 22:00 of
-- the day after the operation date in the owner's timezone (#1168). The
-- overdue leg's conditions are re-checked as of the wake-up instant ($3):
-- planned, non-archived property, the date strictly before the zone's
-- today — auto-pay rules included, the sweep's overdue leg's shape.
SELECT pay.id AS payment_id,
       o.date,
       pay.title,
       o.amount_kopecks,
       p.id AS property_id,
       p.name AS property_name,
       p.address AS property_address,
       p.type AS property_type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/v1/properties/' || p.id::text || '/photo'
                     END, '')::text AS property_photo,
       o.owner_id
FROM operations o
JOIN payments pay ON pay.id = o.payment_id
JOIN properties p ON p.id = o.property_id
JOIN users u ON u.id = o.owner_id
WHERE pay.id = $1::uuid
  AND o.date = $2::date
  AND o.status = 'planned'
  AND p.status IN ('active', 'maintenance')
  AND o.date < ($3::timestamptz AT TIME ZONE u.timezone)::date;

-- name: ListPaymentAutoPaidTargets :many
-- One zone's auto-pay-executed operations as of the zone's today (#1169,
-- карта #1162; решение владельца по гриллингу #1167): planned occurrences
-- of auto-pay rules that the TICK extinguished in their own day —
-- status='paid', paid_source='auto_pay', paid_date = the operation date —
-- on non-archived properties. A manual «Оплатить сейчас» is silent: the
-- stamp is the tick's alone (the owner's decision — the event is about the
-- auto charge, not about any payment). Only the live day: the leg lists
-- date = today — a downtime's missed days are not backfilled (решение
-- владельца, #1167; the overdue leg speaks for the unpaid past instead).
-- The instant gate (#1168): the boundary is the wall clock 10:00 of the
-- operation date — the sweep must not publish ahead of it. The per-payment
-- flag (гейты #1189): молчащие правила (notify_auto_paid = false) мимо —
-- событие «Автоплатёж исполнен» приходит только по разрешившим уведомление.
SELECT pay.id AS payment_id,
       o.date,
       pay.title,
       o.amount_kopecks,
       p.id AS property_id,
       p.name AS property_name,
       p.address AS property_address,
       p.type AS property_type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/v1/properties/' || p.id::text || '/photo'
                     END, '')::text AS property_photo,
       o.owner_id
FROM operations o
JOIN payments pay ON pay.id = o.payment_id
JOIN properties p ON p.id = o.property_id
JOIN users u ON u.id = o.owner_id
WHERE u.timezone = $1
  AND o.date = $2::date
  AND o.status = 'paid'
  AND o.paid_source = 'auto_pay'
  AND o.paid_date = o.date
  AND pay.auto_pay = true
  AND pay.notify_auto_paid
  AND p.status IN ('active', 'maintenance')
  AND ((o.date::timestamp + time '10:00') AT TIME ZONE u.timezone) <= $3::timestamptz
ORDER BY o.id;

-- name: ListPaymentScheduledAutoPaidTargets :many
-- The payments scan's booking list of the auto-paid leg (#1169): the
-- planned occurrences of auto-pay rules on non-archived properties whose
-- boundary — the wall clock 10:00 of the operation date in the owner's
-- timezone — falls in the window (from, until]. The occurrence is planned
-- at booking time; whether the tick (or a manual payment) extinguished it
-- by the wake-up is the delivery-time resolution's call. Only
-- notify_auto_paid-правила бронят (#1189) — молчащие не зовут событие.
SELECT pay.id AS payment_id,
       o.date,
       CAST((o.date::timestamp + time '10:00') AT TIME ZONE u.timezone AS timestamptz) AS fire_at
FROM operations o
JOIN payments pay ON pay.id = o.payment_id
JOIN properties p ON p.id = o.property_id
JOIN users u ON u.id = o.owner_id
WHERE o.status = 'planned'
  AND pay.auto_pay = true
  AND pay.notify_auto_paid
  AND p.status IN ('active', 'maintenance')
  AND ((o.date::timestamp + time '10:00') AT TIME ZONE u.timezone) > $1::timestamptz
  AND ((o.date::timestamp + time '10:00') AT TIME ZONE u.timezone) <= $2::timestamptz
ORDER BY o.id;

-- name: GetScheduledAutoPaidPayment :one
-- The auto-paid boundary job's delivery-time resolution (#1169): the
-- operation as it stands at its 10:00 wake-up. The owner's decision
-- (гриллинг #1167): the event is about the AUTO charge — the tick's stamp
-- (status='paid', paid_source='auto_pay', paid_date = the operation date)
-- is the live predicate; a manual payment of the same occurrence is
-- silent (the owner knows — they paid it). The live day only: an
-- operation whose date is no longer the zone's today (a job awake after
-- the day rolled over) answers no row — a downtime's missed days are not
-- backfilled, the 22:00 overdue leg speaks for the unpaid past; an
-- archived property answers no row. The per-payment flag (#1189): a rule
-- with notify_auto_paid = false answers no row — the flag flipped after
-- the booking silences the standing job.
SELECT pay.id AS payment_id,
       o.date,
       pay.title,
       o.amount_kopecks,
       p.id AS property_id,
       p.name AS property_name,
       p.address AS property_address,
       p.type AS property_type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/v1/properties/' || p.id::text || '/photo'
                     END, '')::text AS property_photo,
       o.owner_id
FROM operations o
JOIN payments pay ON pay.id = o.payment_id
JOIN properties p ON p.id = o.property_id
JOIN users u ON u.id = o.owner_id
WHERE pay.id = $1::uuid
  AND o.date = $2::date
  AND o.status = 'paid'
  AND o.paid_source = 'auto_pay'
  AND o.paid_date = o.date
  AND pay.notify_auto_paid
  AND p.status IN ('active', 'maintenance')
  AND o.date = ($3::timestamptz AT TIME ZONE u.timezone)::date;

-- name: ListPaymentReminderTargets :many
-- One zone's reminder-day operations as of the zone's today (карта #822,
-- #824): planned, on rules with a reminder set, whose reminder day — the
-- operation date minus the rule's lead time (1/3/7) — is exactly today.
-- Auto-pay rules included: напоминание живёт независимо от auto_pay
-- (решение владельца, #823). The dedup key (rule, operation date) keeps
-- the occurrence single even if the lead time changes after firing. The
-- instant gate (#1168): the leg's boundary is the wall clock 10:00 of the
-- reminder day in the owner's timezone — the sweep must not publish ahead
-- of it.
SELECT pay.id AS payment_id,
       o.date,
       pay.title,
       o.amount_kopecks,
       p.id AS property_id,
       p.name AS property_name,
       p.address AS property_address,
       p.type AS property_type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/v1/properties/' || p.id::text || '/photo'
                     END, '')::text AS property_photo,
       o.owner_id
FROM operations o
JOIN payments pay ON pay.id = o.payment_id
JOIN properties p ON p.id = o.property_id
JOIN users u ON u.id = o.owner_id
WHERE u.timezone = $1
  AND pay.reminder_offset_days IS NOT NULL
  AND (o.date - pay.reminder_offset_days) = $2::date
  AND o.status = 'planned'
  AND p.status IN ('active', 'maintenance')
  AND (((o.date - pay.reminder_offset_days)::timestamp + time '10:00') AT TIME ZONE u.timezone) <= $3::timestamptz
ORDER BY o.id;

-- name: ListPaymentScheduledReminderTargets :many
-- The payments scan's booking list of the reminder leg (карта #822, #824):
-- the planned operations of rules with a reminder set whose reminder
-- boundary — the wall clock 10:00 of (operation date − lead time) read in
-- the owner's timezone (#1168, по гриллингу #1167) — falls in the window
-- (from, until]. Auto-pay rules included
-- (the reminder is independent of auto_pay); the wall clock 10:00 is
-- the boundary, a DST day rolls it with the wall clock.
SELECT pay.id AS payment_id,
       o.date,
       CAST(((o.date - pay.reminder_offset_days)::timestamp + time '10:00') AT TIME ZONE u.timezone AS timestamptz) AS fire_at
FROM operations o
JOIN payments pay ON pay.id = o.payment_id
JOIN properties p ON p.id = o.property_id
JOIN users u ON u.id = o.owner_id
WHERE o.status = 'planned'
  AND pay.reminder_offset_days IS NOT NULL
  AND p.status IN ('active', 'maintenance')
  AND ((o.date - pay.reminder_offset_days)::timestamp AT TIME ZONE u.timezone) > $1::timestamptz
  AND ((o.date - pay.reminder_offset_days)::timestamp AT TIME ZONE u.timezone) <= $2::timestamptz
ORDER BY o.id;

-- name: GetScheduledReminderPayment :one
-- The reminder boundary job's delivery-time resolution (карта #822, #824):
-- the operation as it stands at its reminder boundary — the wall clock
-- 10:00 of the reminder day (#1168). Planned, on a
-- non-archived property, and the rule's CURRENT lead time still landing the
-- reminder day on the zone's today ($3) — a lead time changed after the
-- booking (or a job awake after the day rolled over) answers no row, the
-- job finishes without publishing; the new boundary books its own job.
SELECT pay.id AS payment_id,
       o.date,
       pay.title,
       o.amount_kopecks,
       p.id AS property_id,
       p.name AS property_name,
       p.address AS property_address,
       p.type AS property_type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/v1/properties/' || p.id::text || '/photo'
                     END, '')::text AS property_photo,
       o.owner_id
FROM operations o
JOIN payments pay ON pay.id = o.payment_id
JOIN properties p ON p.id = o.property_id
JOIN users u ON u.id = o.owner_id
WHERE pay.id = $1::uuid
  AND o.date = $2::date
  AND o.status = 'planned'
  AND pay.reminder_offset_days IS NOT NULL
  AND (o.date - pay.reminder_offset_days) = ($3::timestamptz AT TIME ZONE u.timezone)::date
  AND p.status IN ('active', 'maintenance');

-- name: ListTaskScanZones :many
-- The tasks scan's sweep targets (карта #734, #750; ADR 0048 p.3): the
-- distinct owner timezones having active dated tasks on non-archived
-- properties or without a property — the only tasks the scan can fire for
-- (the ticks' status canon, same as the rental scan; ADR 0052 — the task
-- without a property stays in its owner's book). Stateless — every run
-- re-lists, no per-zone state is kept.
SELECT DISTINCT u.timezone
FROM tasks t
JOIN users u ON u.id = t.owner_id
LEFT JOIN properties p ON p.id = t.property_id
WHERE t.completed_date IS NULL
  AND t.due_date IS NOT NULL
  AND (t.property_id IS NULL OR p.status IN ('active', 'maintenance'))
ORDER BY u.timezone;

-- name: ListTaskScheduledTargets :many
-- The tasks scan's scheduled leg (issues #750, #777): the active dated
-- tasks whose boundary instant — a timed task's (due_date + due_time), a
-- date-only task's day-after midnight (the wall-clock midnight, a DST day
-- rolls it with the wall clock), each read in the owner's timezone — falls
-- in the window (from, until]. Each one gets a boundary River job booked at
-- its boundary instant; the undated ones stay out (без срока — никогда).
SELECT t.id AS task_id,
       CAST(((t.due_date + COALESCE(t.due_time, '24:00'::time)) AT TIME ZONE u.timezone) AS timestamptz) AS due_at
FROM tasks t
JOIN users u ON u.id = t.owner_id
LEFT JOIN properties p ON p.id = t.property_id
WHERE t.completed_date IS NULL
  AND t.due_date IS NOT NULL
  AND (t.property_id IS NULL OR p.status IN ('active', 'maintenance'))
  AND ((t.due_date + COALESCE(t.due_time, '24:00'::time)) AT TIME ZONE u.timezone) > $1::timestamptz
  AND ((t.due_date + COALESCE(t.due_time, '24:00'::time)) AT TIME ZONE u.timezone) <= $2::timestamptz
ORDER BY t.id;

-- name: ListTaskOverdueTargets :many
-- One zone's overdue tasks as of the sweep's instant (решение #737, тип №4):
-- active, dated, term passed — the timed ones by their term minute
-- (в минуту срока, включительно, tasks/CONTEXT.md «Просрочка»), the
-- date-only ones strictly after the zone's day's end (первый скан после
-- границы суток). Non-archived property or no property at all (ADR 0052).
-- The dedup key (task id) keeps a long-overdue task single — the sweep
-- lists it hourly, the publication inserts nothing.
SELECT t.id AS task_id,
       t.title,
       t.due_date,
       t.due_time,
       t.property_id,
       p.name AS property_name,
       p.address AS property_address,
       p.type AS property_type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/v1/properties/' || p.id::text || '/photo'
                     END, '')::text AS property_photo,
       t.rule_id,
       t.owner_id
FROM tasks t
JOIN users u ON u.id = t.owner_id
LEFT JOIN properties p ON p.id = t.property_id
WHERE u.timezone = $1
  AND t.completed_date IS NULL
  AND t.due_date IS NOT NULL
  AND (t.property_id IS NULL OR p.status IN ('active', 'maintenance'))
  AND (
    (t.due_time IS NOT NULL AND ((t.due_date + t.due_time) AT TIME ZONE u.timezone) <= $2::timestamptz)
    OR (t.due_time IS NULL AND t.due_date < $3::date)
  )
ORDER BY t.id;

-- name: GetScheduledOverdueTask :one
-- The boundary job's delivery-time resolution (issues #750, #777): the task
-- as it stands at its boundary instant. A gone (rule edit removed the stale
-- row), completed, undated, or archived-property task answers no row — the
-- job finishes without publishing. The boundary instant in the owner's
-- timezone (due_at) — a timed task's term minute, a date-only task's
-- day-after midnight — travels for the creation/edit seam (#775), which
-- decides future-vs-past on it; the job wakes at its own instant and needs
-- no clock. Both shapes answer: the kind is one per task, the boundary
-- differs by shape.
SELECT t.id AS task_id,
       t.title,
       t.due_date,
       t.due_time,
       t.property_id,
       p.name AS property_name,
       p.address AS property_address,
       p.type AS property_type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/v1/properties/' || p.id::text || '/photo'
                     END, '')::text AS property_photo,
       t.rule_id,
       t.owner_id,
       CAST(((t.due_date + COALESCE(t.due_time, '24:00'::time)) AT TIME ZONE u.timezone) AS timestamptz) AS due_at
FROM tasks t
JOIN users u ON u.id = t.owner_id
LEFT JOIN properties p ON p.id = t.property_id
WHERE t.id = $1
  AND t.completed_date IS NULL
  AND t.due_date IS NOT NULL
  AND (t.property_id IS NULL OR p.status IN ('active', 'maintenance'));


-- name: GetAccessEventPropertyView :one
-- The access events' property snapshot (#751): the display name and the
-- address line the feed rows' property card carries (EntityRef, #745); the
-- type picks the card avatar's placeholder glyph (карта #1217, #1244), the
-- photo path streams the card's picture when the object has one (#1275). A
-- missing property is a no-row error — the access transitions never fire on
-- a deleted object, a miss is abnormal and fails the publication.
SELECT p.name, p.address, p.type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/v1/properties/' || p.id::text || '/photo'
                     END, '')::text AS property_photo
FROM properties p
WHERE p.id = $1;


-- name: GetTariffEventView :one
-- The tariff events' plan snapshot (#752): the slug the display name resolves
-- from (TariffDisplayName). A missing plan is a no-row error — the billing
-- events never fire on a dropped tariff, a miss is abnormal and fails the
-- publication.
SELECT t.name
FROM tariffs t
WHERE t.id = $1;
