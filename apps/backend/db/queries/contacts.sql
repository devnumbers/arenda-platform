-- Contacts context queries: CRUD and search over the owner's contact book
-- (ADR 0051, ticket #506). Reads and writes are scoped by the data owner
-- (ADR 0028) — the service has already resolved the actor's role. The by-id
-- read alone is unscoped on purpose: the card itself carries the property
-- binding the authorization gates on, so the service loads first and
-- authorizes before anything travels out.

-- name: GetContactByID :one
SELECT * FROM contacts
WHERE id = $1;

-- name: ListContacts :many
-- The owner's slice per the query scope: 'all' — the whole book,
-- 'without_property' — the unbound cards, 'property' — one property's cards
-- (property_id must be set for it). search ('' = no filter) is a
-- case-insensitive substring match over the name fields, role, phone, email
-- and messenger username; the application layer escapes the ILIKE
-- metacharacters (ESCAPE '\').
SELECT *
FROM contacts
WHERE owner_id = $1
  AND (
        sqlc.arg('scope')::text = 'all'
        OR (sqlc.arg('scope')::text = 'without_property' AND property_id IS NULL)
        OR (sqlc.arg('scope')::text = 'property' AND property_id = sqlc.arg('property_id')::uuid)
      )
  AND (
        sqlc.arg('search')::text = ''
        OR concat_ws(' ', first_name, last_name, patronymic, role, phone, email, messenger_username)
           ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
      )
ORDER BY created_at ASC, id ASC;

-- name: InsertContact :one
INSERT INTO contacts (id, owner_id, property_id, first_name, last_name, patronymic,
                      role, phone, email, messenger_name, messenger_username, note)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING *;

-- name: UpdateContact :one
-- The full editable-field rewrite keyed by (id, owner_id): zero rows means
-- the card is gone between the service's read and this write (mapped to
-- ErrNotFound, taking the audit entry down with it).
UPDATE contacts SET
    property_id = $3,
    first_name = $4,
    last_name = $5,
    patronymic = $6,
    role = $7,
    phone = $8,
    email = $9,
    messenger_name = $10,
    messenger_username = $11,
    note = $12
WHERE id = $1 AND owner_id = $2
RETURNING *;

-- name: DeleteContact :execrows
DELETE FROM contacts
WHERE id = $1 AND owner_id = $2;

-- name: ListContactsAdmin :many
-- The admin read of one property's contacts (ADR 0051 consequences): the
-- bound cards only — an unbound contact belongs to no property card.
SELECT *
FROM contacts
WHERE property_id = $1
ORDER BY created_at ASC, id ASC
LIMIT $2 OFFSET $3;

-- name: GetContactPropertyRef :one
-- The contacts view of the property a use case targets: just the data owner
-- whose book the property-bound cards belong to (ADR 0028).
SELECT id, owner_id FROM properties WHERE id = $1;

-- name: CountContactsAdmin :one
SELECT COUNT(*) FROM contacts
WHERE property_id = $1;
