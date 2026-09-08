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
