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
SELECT role FROM property_members
WHERE property_id = $1 AND user_id = $2;

-- name: ListPropertyMembers :many
SELECT * FROM property_members
WHERE property_id = $1
ORDER BY created_at ASC;

-- name: ListPropertyMembersByUser :many
SELECT property_id, role FROM property_members
WHERE user_id = $1;

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
  AND p.owner_id = sqlc.arg('owner_id')::uuid;

-- name: ListAccessibleOwners :many
SELECT DISTINCT p.owner_id
FROM property_members m
JOIN properties p ON p.id = m.property_id
WHERE m.user_id = sqlc.arg('user_id')::uuid;
