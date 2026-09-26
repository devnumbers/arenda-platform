-- Contacts context queries: CRUD and search over the owner's contact book
-- (ADR 0054, ticket #506). Reads and writes are scoped by the data owner
-- (ADR 0028) — the service has already resolved the actor's role. The by-id
-- read alone is unscoped on purpose: the card itself carries the property
-- binding the authorization gates on, so the service loads first and
-- authorizes before anything travels out.

-- name: GetContactByID :one
SELECT * FROM contacts
WHERE id = $1;

-- name: ListContacts :many
-- The actor's visible slice per the query scope (ADR 0054, ADR 0028):
-- 'all' — the flat book: the actor's own cards plus the cards bound to
-- properties the actor owns or shares with an active membership (the merged
-- visibility the global book page reads);
-- 'without_property' — the actor's own unbound cards;
-- 'property' — the cards bound to one property whatever book they live in:
-- visibility is driven by the binding (the book owner never changes on a
-- move), and the service has already gated the actor's view capability on
-- the property. property_id must be set for the property scope.
-- search ('' = no filter) is a case-insensitive substring match over the
-- name fields, role, phone, email and messenger username, glued by
-- contacts_search_text — the same IMMUTABLE expression the trigram index
-- (migration 000124) is built on; the application layer escapes the ILIKE
-- metacharacters (ESCAPE '\').
-- sort 'name' orders by the display name; 'property' — by the bound
-- property's name, unbound cards first in both directions («Общие
-- контакты»), contact name ordering inside the groups. Both keys use the
-- Russian ICU collation to match the client's letter grouping; id ties off.
-- 'created' orders by the creation moment (ticket #847 — the «свежие
-- контакты сверху» promise server-side): the key is immutable, so a walked
-- card never moves across the window boundary.
--
-- The page walks the listing's own order by keyset (ticket #600): the
-- window resumes strictly after the (sort key, id) the previous page ended
-- on, so cards created or deleted between loads never duplicate or drop.
-- The sort keys are mutable (display name, property binding): a rename or
-- rebind of a card the walk has already passed can move it across the
-- window boundary — inherent to the visible name/property sort. The
-- predicate mirrors the ORDER BY branch by branch — the same CASE-gated
-- keys, the same ICU collations, the unbound-flag leading the property sort
-- and id tying off ascending in both directions. The cursor blob carries
-- its sort/order: the application rejects a cursor echoed under another
-- walk. All cursor args travel together; NULL (no cursor) reads from the
-- beginning.
SELECT c.*, p.name AS property_name
FROM contacts c
LEFT JOIN properties p ON p.id = c.property_id
WHERE (
       (sqlc.arg('scope')::text = 'all' AND (
          c.owner_id = sqlc.arg('actor_id')::uuid
          OR (
            c.property_id IS NOT NULL
            AND (
                  EXISTS (
                    SELECT 1 FROM properties op
                    WHERE op.id = c.property_id
                      AND op.owner_id = sqlc.arg('actor_id')::uuid
                  )
               OR EXISTS (
                    SELECT 1 FROM property_members pm
                    WHERE pm.property_id = c.property_id
                      AND pm.user_id = sqlc.arg('actor_id')::uuid
                      AND pm.status = 'active'
                  )
            )
          )
       ))
    OR (sqlc.arg('scope')::text = 'without_property'
        AND c.owner_id = sqlc.arg('actor_id')::uuid
        AND c.property_id IS NULL)
    OR (sqlc.arg('scope')::text = 'property'
        AND c.property_id = sqlc.arg('property_id')::uuid)
      )
  AND (
        sqlc.arg('search')::text = ''
        OR contacts_search_text(c.first_name, c.last_name, c.patronymic, c.role, c.phone, c.email, c.messenger_username)
           ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
      )
  AND (
        sqlc.narg('after_id')::uuid IS NULL
        OR (sqlc.arg('sort')::text = 'name'
            AND sqlc.arg('order')::text = 'asc'
            AND (concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" > sqlc.narg('after_name')::text
                 OR (concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" = sqlc.narg('after_name')::text
                     AND c.id > sqlc.narg('after_id')::uuid)))
        OR (sqlc.arg('sort')::text = 'name'
            AND sqlc.arg('order')::text = 'desc'
            AND (concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" < sqlc.narg('after_name')::text
                 OR (concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" = sqlc.narg('after_name')::text
                     AND c.id > sqlc.narg('after_id')::uuid)))
        OR (sqlc.arg('sort')::text = 'property'
            AND sqlc.arg('order')::text = 'asc'
            AND (
                  (c.property_id IS NULL
                   AND sqlc.narg('after_unbound')::bool = TRUE
                   AND (concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" > sqlc.narg('after_name')::text
                        OR (concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" = sqlc.narg('after_name')::text
                            AND c.id > sqlc.narg('after_id')::uuid)))
               OR (c.property_id IS NOT NULL
                   AND (sqlc.narg('after_unbound')::bool = TRUE
                        OR (p.name COLLATE "ru-RU-x-icu" > sqlc.narg('after_property_name')::text
                            OR (p.name COLLATE "ru-RU-x-icu" = sqlc.narg('after_property_name')::text
                                AND (concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" > sqlc.narg('after_name')::text
                                     OR (concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" = sqlc.narg('after_name')::text
                                         AND c.id > sqlc.narg('after_id')::uuid))))))
                       ))
        OR (sqlc.arg('sort')::text = 'property'
            AND sqlc.arg('order')::text = 'desc'
            AND (
                  (c.property_id IS NULL
                   AND sqlc.narg('after_unbound')::bool = TRUE
                   AND (concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" < sqlc.narg('after_name')::text
                        OR (concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" = sqlc.narg('after_name')::text
                            AND c.id > sqlc.narg('after_id')::uuid)))
               OR (c.property_id IS NOT NULL
                   AND (sqlc.narg('after_unbound')::bool = TRUE
                        OR (p.name COLLATE "ru-RU-x-icu" < sqlc.narg('after_property_name')::text
                            OR (p.name COLLATE "ru-RU-x-icu" = sqlc.narg('after_property_name')::text
                                AND (concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" < sqlc.narg('after_name')::text
                                     OR (concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" = sqlc.narg('after_name')::text
                                         AND c.id > sqlc.narg('after_id')::uuid))))))
                       ))
        OR (sqlc.arg('sort')::text = 'created'
            AND sqlc.arg('order')::text = 'asc'
            AND (c.created_at > sqlc.narg('after_created_at')::timestamptz
                 OR (c.created_at = sqlc.narg('after_created_at')::timestamptz
                     AND c.id > sqlc.narg('after_id')::uuid)))
        OR (sqlc.arg('sort')::text = 'created'
            AND sqlc.arg('order')::text = 'desc'
            AND (c.created_at < sqlc.narg('after_created_at')::timestamptz
                 OR (c.created_at = sqlc.narg('after_created_at')::timestamptz
                     AND c.id > sqlc.narg('after_id')::uuid)))
      )
ORDER BY
  CASE WHEN sqlc.arg('sort')::text = 'property'
       THEN (c.property_id IS NULL) END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'name' AND sqlc.arg('order')::text = 'asc'
       THEN concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'name' AND sqlc.arg('order')::text = 'desc'
       THEN concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'property' AND sqlc.arg('order')::text = 'asc'
       THEN p.name COLLATE "ru-RU-x-icu" END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'property' AND sqlc.arg('order')::text = 'asc'
       THEN concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'property' AND sqlc.arg('order')::text = 'desc'
       THEN p.name COLLATE "ru-RU-x-icu" END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'property' AND sqlc.arg('order')::text = 'desc'
       THEN concat_ws(' ', c.first_name, c.last_name, c.patronymic) COLLATE "ru-RU-x-icu" END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'created' AND sqlc.arg('order')::text = 'asc'
       THEN c.created_at END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'created' AND sqlc.arg('order')::text = 'desc'
       THEN c.created_at END DESC,
  c.id ASC
LIMIT sqlc.arg('page_limit');

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
-- The admin read of one property's contacts (ADR 0054 consequences): the
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
