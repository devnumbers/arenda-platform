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

-- name: ListAccessibleOwners :many
SELECT DISTINCT p.owner_id
FROM property_members m
JOIN properties p ON p.id = m.property_id
WHERE m.user_id = sqlc.arg('user_id')::uuid
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
SELECT * FROM property_members
WHERE user_id = $1 AND status = 'suspended'
ORDER BY suspended_at ASC NULLS LAST, updated_at DESC;

-- name: CountActiveMembersByUser :one
SELECT COUNT(*) FROM property_members
WHERE user_id = $1 AND status = 'active';

-- name: CountSuspendedMembersByUser :one
SELECT COUNT(*) FROM property_members
WHERE user_id = $1 AND status = 'suspended';

-- name: ListActiveMembersByPropertyOwner :many
SELECT m.*
FROM property_members m
JOIN properties p ON p.id = m.property_id
WHERE p.owner_id = $1 AND m.status = 'active'
ORDER BY m.user_id, m.updated_at DESC;

-- name: ListActiveMembersByUser :many
SELECT * FROM property_members
WHERE user_id = $1 AND status = 'active';

-- name: CreatePropertyMemberWithStatus :one
INSERT INTO property_members (id, property_id, user_id, role, granted_by, status)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;
