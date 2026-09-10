-- Rentals context queries: the rental CRUD and the list ordering (ADR 0053
-- §4, ticket #529). Reads and writes are scoped by the data owner (ADR 0028:
-- SQL filters by scope, the policy port has already resolved the actor's
-- role); the nested path rental→property is enforced in the WHERE clause.
-- The tenant is embedded by a LEFT JOIN: after the contact's deletion the
-- link is NULL and the tenant is simply absent (решение #528).

-- name: GetRentalByID :one
-- One rental by id within the owner's scope on the given property, with the
-- tenant's contact fields resolved for the embedded tenant view.
SELECT r.id,
       r.owner_id,
       r.property_id,
       r.payment_id,
       r.contact_id,
       r.start_date,
       r.planned_end_date,
       r.completed_date,
       r.utilities,
       r.deposit_kopecks,
       r.commission_kopecks,
       r.deposit_return_kopecks,
       r.deposit_return_comment,
       r.comment,
       r.created_at,
       r.updated_at,
       c.first_name  AS tenant_first_name,
       c.last_name   AS tenant_last_name,
       c.phone       AS tenant_phone
FROM rentals r
LEFT JOIN contacts c ON c.id = r.contact_id
WHERE r.id = $1 AND r.owner_id = $2 AND r.property_id = $3;

-- name: ListRentalsByProperty :many
-- The property's rentals: unfinished first (newest start on top), then the
-- completed ones by completion date, fresh on top (ADR 0053 §4).
SELECT r.id,
       r.owner_id,
       r.property_id,
       r.payment_id,
       r.contact_id,
       r.start_date,
       r.planned_end_date,
       r.completed_date,
       r.utilities,
       r.deposit_kopecks,
       r.commission_kopecks,
       r.deposit_return_kopecks,
       r.deposit_return_comment,
       r.comment,
       r.created_at,
       r.updated_at,
       c.first_name  AS tenant_first_name,
       c.last_name   AS tenant_last_name,
       c.phone       AS tenant_phone
FROM rentals r
LEFT JOIN contacts c ON c.id = r.contact_id
WHERE r.owner_id = $1 AND r.property_id = $2
ORDER BY r.completed_date IS NOT NULL,
         r.completed_date DESC,
         r.start_date DESC,
         r.id;

-- name: InsertRental :exec
-- The id, owner and payment link are app-side (UUIDv7, the denormalized
-- scope owner, the gateway-minted rent payment).
INSERT INTO rentals (
    id, owner_id, property_id, payment_id, contact_id,
    start_date, planned_end_date, utilities,
    deposit_kopecks, commission_kopecks, comment
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: UpdateRental :exec
-- Partial PATCH is resolved by the application layer; the statement always
-- writes the full editable set. The start date is not editable (ADR 0053
-- §3); completion and the deposit return belong to CompleteRental.
UPDATE rentals
SET contact_id = $3,
    planned_end_date = $4,
    utilities = $5,
    deposit_kopecks = $6,
    commission_kopecks = $7,
    comment = $8
WHERE id = $1 AND owner_id = $2;

-- name: CompleteRental :execrows
-- Завершение: the completion date and the optional deposit return land in
-- one UPDATE. The one-unfinished-per-property index releases here; the
-- caller has proven the rental unfinished inside the same transaction.
UPDATE rentals
SET completed_date = $3,
    deposit_return_kopecks = $4,
    deposit_return_comment = $5
WHERE id = $1 AND owner_id = $2;

-- name: DeleteRental :execrows
-- Hard delete of the rental row; it must run before the payment's own delete
-- (the RESTRICT FK releases only after the rental row is gone, ADR 0053 §3).
DELETE FROM rentals WHERE id = $1 AND owner_id = $2;

-- name: ExistsUnfinishedRental :one
-- The create-time app check for invariant №12 (одна незавершённая на
-- объекте): run inside the transaction under the property lock; the partial
-- unique index backstops the race.
SELECT EXISTS (
    SELECT 1 FROM rentals
    WHERE owner_id = $1 AND property_id = $2 AND completed_date IS NULL
);

-- name: ListUnfinishedRentalsByPropertyIDs :many
-- The occupancy projection of the properties list (ticket #585, резолюция
-- #584): the unfinished rentals of the given properties in one batched read
-- — exactly one per property (invariant №12), so the result over a listed
-- property is either one row or none. The status itself computes in Go
-- against the data owner's today (ADR 0048), from the same dates the
-- rentals responses report.
SELECT property_id, owner_id, start_date, planned_end_date
FROM rentals
WHERE completed_date IS NULL
  AND property_id = ANY(@property_ids::uuid[]);
