-- name: CreatePropertyMember :one
INSERT INTO property_members (id, property_id, user_id, role, granted_by)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetPropertyMember :one
SELECT * FROM property_members
WHERE id = $1 AND property_id = $2;

-- name: GetPropertyMemberByPropertyAndUser :one
SELECT * FROM property_members
WHERE property_id = $1 AND user_id = $2;

-- name: GetPropertyMemberRole :one
-- Returns the role of an ACTIVE membership only. A suspended membership is
-- treated as no access: the SQL-level status filter makes a suspended
-- recipient indistinguishable from a non-member for authorization purposes
-- (issue #158, T4). The UI distinguishes suspended via a dedicated screen (T9).
SELECT role FROM property_members
WHERE property_id = $1 AND user_id = $2 AND status = 'active';

-- name: ListPropertyMembers :many
SELECT * FROM property_members
WHERE property_id = $1
ORDER BY created_at ASC;

-- name: ListPropertyMembersByUser :many
SELECT property_id, role FROM property_members
WHERE user_id = $1 AND status = 'active';

-- name: UpdatePropertyMemberRole :one
UPDATE property_members SET role = $3
WHERE id = $1 AND property_id = $2
RETURNING *;

-- name: DeletePropertyMember :exec
DELETE FROM property_members
WHERE id = $1 AND property_id = $2;

-- name: GetMaxMemberRoleByOwner :one
SELECT COALESCE(
  MAX(
    CASE role
      WHEN 'full_access' THEN 2
      WHEN 'viewer' THEN 1
      ELSE 0
    END
  ),
  -1
)::int AS max_role
FROM property_members m
JOIN properties p ON p.id = m.property_id
WHERE m.user_id = sqlc.arg('user_id')::uuid
  AND p.owner_id = sqlc.arg('owner_id')::uuid
  AND m.status = 'active';

-- name: SuspendPropertyMember :exec
UPDATE property_members
SET status = 'suspended', suspended_at = now()
WHERE id = $1 AND property_id = $2;

-- name: ReactivatePropertyMember :one
UPDATE property_members
SET status = 'active', suspended_at = NULL
WHERE id = $1 AND property_id = $2
RETURNING *;

-- name: ListSuspendedMembersByUser :many
-- The FIFO recovery queue. Memberships on archived properties are excluded:
-- an archived object does not occupy a recipient slot (issue #163), so a free
-- slot must not be wasted on them; they re-enter the selection on unarchive.
-- Predicate and order are shared with ListSuspendedSharedWithOwner below
-- (the list's blur-card placeholders, ticket #702) — change them together.
SELECT m.*
FROM property_members m
JOIN properties p ON p.id = m.property_id
WHERE m.user_id = $1 AND m.status = 'suspended' AND p.status != 'archived'
ORDER BY m.suspended_at ASC NULLS LAST, m.updated_at DESC;

-- name: ListSuspendedSharedWithOwner :many
-- The blur-cards of the recipient's main property list (ticket #702):
-- suspended memberships on non-archived properties — the same predicate and
-- FIFO order the hidden-shared count used (#158 T4, #163) — each with the
-- object's own card data (title, address — the card renders for real under
-- the blur, Figma 2213-99113) and the data owner id for the reason sheet's
-- contact row. Owner display data and the first photo resolve through the
-- access context's follow-up lookups; the access SQL never joins users.
SELECT m.property_id, m.role, p.name, p.address, p.owner_id
FROM property_members m
JOIN properties p ON p.id = m.property_id
WHERE m.user_id = $1 AND m.status = 'suspended' AND p.status != 'archived'
ORDER BY m.suspended_at ASC NULLS LAST, m.updated_at DESC;

-- name: CountActiveMembersByUser :one
-- Occupied recipient tariff slots: memberships on archived properties do not
-- occupy a slot (issue #163).
SELECT COUNT(*)
FROM property_members m
JOIN properties p ON p.id = m.property_id
WHERE m.user_id = $1 AND m.status = 'active' AND p.status != 'archived';

-- name: ListActiveMembersByPropertyOwner :many
SELECT m.*
FROM property_members m
JOIN properties p ON p.id = m.property_id
WHERE p.owner_id = $1 AND m.status = 'active'
ORDER BY m.user_id, m.updated_at DESC;

-- name: ListActiveMembersByUser :many
-- The recipient's shared-pool entries for slot accounting. Memberships on
-- archived properties are excluded: an archived object does not occupy a
-- recipient slot (issue #163).
SELECT m.*
FROM property_members m
JOIN properties p ON p.id = m.property_id
WHERE m.user_id = $1 AND m.status = 'active' AND p.status != 'archived';

-- name: CreatePropertyMemberWithStatus :one
INSERT INTO property_members (id, property_id, user_id, role, granted_by, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;
