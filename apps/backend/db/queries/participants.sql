-- The «Участник (владельца)» read model (issue #693): an aggregate over
-- property_members ∪ property_member_invitations with no table of its own
-- (chart decision of map #692). The reading actor's scope is the set of
-- non-archived properties the actor owns or manages as an active full_access
-- member; archived properties stay invisible like everywhere else on the
-- platform (issue #163). The scope doubles as the authorization: rows outside
-- it never leave the database.

-- name: ListParticipantScopeProperties :many
SELECT p.id, p.owner_id, p.name AS title
FROM properties p
WHERE p.status <> 'archived'
  AND (
    p.owner_id = sqlc.arg('actor_id')::uuid
    OR EXISTS (
      SELECT 1 FROM property_members m
      WHERE m.property_id = p.id
        AND m.user_id = sqlc.arg('actor_id')::uuid
        AND m.status = 'active'
        AND m.role = 'full_access'
    )
  )
ORDER BY p.name ASC, p.id ASC;

-- name: ListParticipantMembershipsByProperties :many
SELECT m.property_id, m.user_id, m.role, m.status
FROM property_members m
WHERE m.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[])
ORDER BY m.created_at ASC;

-- name: ListParticipantInvitationsByProperties :many
SELECT i.property_id, i.email, i.role
FROM property_member_invitations i
WHERE i.property_id = ANY(string_to_array(sqlc.arg('property_ids')::text, ',')::uuid[])
ORDER BY i.created_at ASC;

-- The mutation side of the owner's participant aggregate (issue #694):
-- «Отозвать и удалить» enumerates one person's legs within the acting
-- actor's manage scope. Unlike the read scope above, archived properties are
-- INCLUDED here: revoking access keeps working on archived objects (issue
-- #163) — an archived leg must not survive a full removal, or the person
-- would silently reappear on unarchive. The scope predicate is the
-- authorization: rows outside it never leave the database.

-- name: ListParticipantMembershipsForRemoval :many
SELECT m.id, m.property_id, m.user_id, m.status
FROM property_members m
JOIN properties p ON p.id = m.property_id
WHERE m.user_id = sqlc.arg('person_id')::uuid
  AND (
    p.owner_id = sqlc.arg('actor_id')::uuid
    OR EXISTS (
      SELECT 1 FROM property_members am
      WHERE am.property_id = m.property_id
        AND am.user_id = sqlc.arg('actor_id')::uuid
        AND am.status = 'active'
        AND am.role = 'full_access'
    )
  )
ORDER BY m.created_at ASC;

-- name: ListParticipantInvitationsForRemoval :many
SELECT i.id, i.property_id, i.email
FROM property_member_invitations i
JOIN properties p ON p.id = i.property_id
WHERE lower(i.email) = lower(sqlc.arg('person_email')::text)
  AND (
    p.owner_id = sqlc.arg('actor_id')::uuid
    OR EXISTS (
      SELECT 1 FROM property_members am
      WHERE am.property_id = p.id
        AND am.user_id = sqlc.arg('actor_id')::uuid
        AND am.status = 'active'
        AND am.role = 'full_access'
    )
  )
ORDER BY i.created_at ASC;
