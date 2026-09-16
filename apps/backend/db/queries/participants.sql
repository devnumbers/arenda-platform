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
