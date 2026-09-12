-- name: CreateProperty :one
INSERT INTO properties (id, owner_id, name, type, address, description, attributes, status)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: GetPropertyByIDAndOwner :one
SELECT properties.*,
       ((SELECT COUNT(*) FROM property_members pm
         WHERE pm.property_id = properties.id) +
       (SELECT COUNT(*) FROM property_member_invitations pmi
         WHERE pmi.property_id = properties.id))::bigint AS members_count
FROM properties
WHERE properties.id = $1 AND properties.owner_id = $2;

-- name: GetPropertyByIDAndOwnerForUpdate :one
SELECT * FROM properties WHERE id = $1 AND owner_id = $2
FOR UPDATE;

-- name: GetPropertyStatusByOwner :one
SELECT status FROM properties
WHERE id = $1 AND owner_id = $2;

-- name: GetPropertyByID :one
-- Unscoped lookup by id. Used by the policy/access layer (T3, issue #156) to
-- resolve the data owner for authorization before applying a scope, and by the
-- properties list to load shared properties (whose owner_id differs from the
-- actor). Read-only; callers must never leak existence to actors without a view
-- capability (the application maps "no access" to ErrNotFound to preserve
-- object privacy). members_count is the shared-access participant count
-- (membership rows of any status plus pending email invitations, issue #163).
SELECT properties.*,
       ((SELECT COUNT(*) FROM property_members pm
         WHERE pm.property_id = properties.id) +
       (SELECT COUNT(*) FROM property_member_invitations pmi
         WHERE pmi.property_id = properties.id))::bigint AS members_count
FROM properties
WHERE properties.id = $1;

-- name: GetPropertyByIDForUpdate :one
-- Unscoped pessimistic-lock lookup by id, for write paths that resolve access
-- via the policy port before applying scope = owner_id (T3, issue #156).
SELECT * FROM properties WHERE id = $1 FOR UPDATE;

-- name: ListActivePropertiesByOwner :many
-- The main list's order (ticket #577): the pinned first — among themselves
-- by the pin time (the first pin stays on top, a re-pin never shifts the
-- order), then the unpinned by updated_at DESC. The application re-applies
-- the same rule over the merged list (shared properties arrive appended),
-- so keep the two passes in sync (PropertyService.pinnedFirst).
SELECT properties.*,
       ((SELECT COUNT(*) FROM property_members pm
         WHERE pm.property_id = properties.id) +
       (SELECT COUNT(*) FROM property_member_invitations pmi
         WHERE pmi.property_id = properties.id))::bigint AS members_count
FROM properties
WHERE properties.owner_id = $1 AND properties.status IN ('active', 'maintenance')
ORDER BY properties.pinned_at, properties.updated_at DESC;

-- name: ListArchivedPropertiesByOwner :many
SELECT properties.*,
       ((SELECT COUNT(*) FROM property_members pm
         WHERE pm.property_id = properties.id) +
       (SELECT COUNT(*) FROM property_member_invitations pmi
         WHERE pmi.property_id = properties.id))::bigint AS members_count
FROM properties
WHERE properties.owner_id = $1 AND properties.status = 'archived'
ORDER BY properties.updated_at DESC;

-- name: SearchVisibleProperties :many
-- The search endpoint's window (ticket #601): the actor's visible
-- non-archived properties — own plus actively shared (the merged visibility
-- of the main list, ADR 0028) — matching the search as a case-insensitive
-- substring over the name or the address (the store escapes the ILIKE
-- metacharacters; the trgm indexes of migration 000124 serve both fields).
--
-- The walk is keyset over the display name and id (ticket #597's pattern):
-- the window resumes strictly after the (name, id) the previous page ended
-- on, so rows created, deleted or renamed between loads never duplicate or
-- drop. The name sort mirrors the hub's own default (the client orders the
-- list by name) with the Russian ICU collation matching the contacts book's
-- letter order (#600); id ties off. A rename moving a row across the window
-- boundary is inherent to the visible-name sort. The cursor args travel
-- together; NULL (no cursor) reads from the beginning.
-- access_role names the actor's role on the row: 'owner' for own
-- properties, the active membership's role for shared ones (T11) — the
-- LEFT JOIN row is unique per (property, user).
SELECT p.*,
       ((SELECT COUNT(*) FROM property_members pm
         WHERE pm.property_id = p.id) +
       (SELECT COUNT(*) FROM property_member_invitations pmi
         WHERE pmi.property_id = p.id))::bigint AS members_count,
       CASE WHEN p.owner_id = sqlc.arg('actor')::uuid
            THEN 'owner'
            ELSE pm.role END::text AS access_role
FROM properties p
LEFT JOIN property_members pm
       ON pm.property_id = p.id
      AND pm.user_id = sqlc.arg('actor')::uuid
      AND pm.status = 'active'
WHERE (
       p.owner_id = sqlc.arg('actor')::uuid
       OR EXISTS (
            SELECT 1 FROM property_members vpm
            WHERE vpm.property_id = p.id
              AND vpm.user_id = sqlc.arg('actor')::uuid
              AND vpm.status = 'active'
          )
      )
  AND p.status IN ('active', 'maintenance')
  AND (p.name ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\'
       OR p.address ILIKE '%' || sqlc.arg('search')::text || '%' ESCAPE '\')
  AND (sqlc.narg('after_name')::text IS NULL
       OR (p.name COLLATE "ru-RU-x-icu" > sqlc.narg('after_name')::text
           OR (p.name COLLATE "ru-RU-x-icu" = sqlc.narg('after_name')::text
               AND p.id > sqlc.narg('after_id')::uuid)))
ORDER BY p.name COLLATE "ru-RU-x-icu" ASC, p.id ASC
LIMIT sqlc.arg('page_limit')::int;

-- name: UpdateProperty :one
UPDATE properties
SET name = $3, type = $4, address = $5, description = $6, attributes = $7, status = $8
WHERE id = $1 AND owner_id = $2
RETURNING *;

-- name: ArchiveProperty :one
-- Archiving clears the pin in the same UPDATE: the schema CHECK
-- (properties_pinned_at_check) demands it, and an unarchived object returns
-- unpinned (ticket #577).
UPDATE properties SET status = 'archived', pinned_at = NULL
WHERE id = $1 AND owner_id = $2
RETURNING *;

-- name: UnarchiveProperty :one
UPDATE properties SET status = 'active'
WHERE id = $1 AND owner_id = $2
RETURNING *;

-- name: CountActivePropertiesByOwner :one
SELECT COUNT(*) FROM properties
WHERE owner_id = $1 AND status IN ('active', 'maintenance');

-- name: SetPropertyPin :one
-- Atomic PUT pin (ticket #577, the PUT favorite's canon #461: no
-- read-modify-write). The value — a moment or NULL — is resolved by the
-- application under the row lock (a re-pin keeps the original pin time);
-- existence and the edit capability are proven there too. The scope is the
-- property's owner: the actor may be the full-access member. The updated
-- row travels back for the response.
UPDATE properties
SET pinned_at = sqlc.arg('pinned_at')::timestamptz
WHERE id = sqlc.arg('id')::uuid AND owner_id = sqlc.arg('owner_id')::uuid
RETURNING *;

-- name: ListPropertiesAdmin :many
SELECT p.*, u.phone AS owner_phone, u.phone_encrypted AS owner_phone_encrypted
FROM properties p
JOIN users u ON p.owner_id = u.id
WHERE (sqlc.arg('owner_id')::uuid IS NULL OR p.owner_id = sqlc.arg('owner_id')::uuid)
  AND (sqlc.arg('status')::text = '' OR p.status = sqlc.arg('status')::text)
  AND (sqlc.arg('q')::text = '' OR p.name ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\' OR p.address ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\')
ORDER BY
  CASE WHEN sqlc.arg('sort')::text = 'name' AND sqlc.arg('order')::text = 'asc' THEN p.name END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'name' AND sqlc.arg('order')::text = 'desc' THEN p.name END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'createdAt' AND sqlc.arg('order')::text = 'asc' THEN p.created_at END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'createdAt' AND sqlc.arg('order')::text = 'desc' THEN p.created_at END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'updatedAt' AND sqlc.arg('order')::text = 'asc' THEN p.updated_at END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'updatedAt' AND sqlc.arg('order')::text = 'desc' THEN p.updated_at END DESC,
  CASE WHEN sqlc.arg('sort')::text = 'status' AND sqlc.arg('order')::text = 'asc' THEN p.status END ASC,
  CASE WHEN sqlc.arg('sort')::text = 'status' AND sqlc.arg('order')::text = 'desc' THEN p.status END DESC,
  CASE WHEN sqlc.arg('sort')::text = '' THEN p.updated_at END DESC,
  p.id DESC
LIMIT sqlc.arg('limit')::int OFFSET sqlc.arg('offset')::int;

-- name: CountPropertiesAdmin :one
SELECT COUNT(*) FROM properties
WHERE (sqlc.arg('owner_id')::uuid IS NULL OR owner_id = sqlc.arg('owner_id')::uuid)
  AND (sqlc.arg('status')::text = '' OR status = sqlc.arg('status')::text)
  AND (sqlc.arg('q')::text = '' OR name ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\' OR address ILIKE '%' || sqlc.arg('q')::text || '%' ESCAPE '\');

-- name: GetPropertyByIDAdmin :one
SELECT p.*, u.phone AS owner_phone, u.phone_encrypted AS owner_phone_encrypted
FROM properties p
JOIN users u ON p.owner_id = u.id
WHERE p.id = $1;

-- name: CountActivePropertiesByOwnerAdmin :one
SELECT COUNT(*) FROM properties
WHERE owner_id = $1 AND status IN ('active', 'maintenance');

-- name: CountArchivedPropertiesByOwnerAdmin :one
SELECT COUNT(*) FROM properties
WHERE owner_id = $1 AND status = 'archived';

-- name: GetPropertiesStatsAdmin :one
-- "active" mirrors CountActivePropertiesByOwnerAdmin: active plus maintenance.
SELECT
  COUNT(*) FILTER (WHERE status IN ('active', 'maintenance')) AS active_count,
  COUNT(*) FILTER (WHERE status = 'archived') AS archived_count
FROM properties;

-- name: DeleteProperty :exec
DELETE FROM properties
WHERE id = $1 AND owner_id = $2;
