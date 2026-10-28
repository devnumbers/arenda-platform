-- The «Участник (владельца)» read model (issue #693): an aggregate over
-- property_members ∪ property_member_invitations with no table of its own
-- (chart decision of map #692). The reading actor's scope is the set of
-- non-archived properties the actor owns; since the D8 decision of the
-- sharing effort (issue #1304) member management — and with it the hub — is
-- the owner's prerogative, so a full_access member of a foreign object sees
-- none of its owner's participants; archived properties stay invisible like
-- everywhere else on the platform (issue #163). The scope doubles as the
-- authorization: rows outside it never leave the database.
--
-- THE MANAGE-SCOPE PREDICATE (the «actor_can_manage» canon): owner-only
-- since D8 (issue #1304; migration 000157) is the authorization of the five
-- scope queries in this file (read scope, both per-person removal listings,
-- both owner-wide removal listings of the global revoke #1310; the two
-- ByProperties listings filter by the already-scoped id list). The predicate
-- lives in one place — the SQL function actor_can_manage (migration 000135,
-- issue #794) — which the queries below call; before #794 sqlc could not
-- share the text between query bodies, so the predicate was kept as three
-- byte-identical copies pinned by the matrix test.
-- TestParticipantRepository_ManageScopePredicateMatrix runs the same actor
-- verdict across these queries and stays the gate: semantic drift of the
-- function fails there instead of opening a silent privacy hole.

-- name: ListParticipantScopeProperties :many
-- The scope rows carry the leg avatar pair (решение #1286, остаток #1244
-- п.3): the property type — the placeholder glyph's key — and the photo's
-- same-origin streaming path (ADR 0065), so the participant read model's
-- legs are self-sufficient and the client needs no /properties join (a cold
-- cache no longer renders BoldHome).
SELECT p.id, p.owner_id, p.name AS title, p.type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/properties/' || p.id::text || '/photo'
                     || '?v=' || left(encode(sha256(p.photo_key::bytea), 'hex'), 32)
                     END, '')::text AS photo_path,
       COALESCE(p.photo_blur, '')::text AS photo_blur
FROM properties p
WHERE p.status <> 'archived'
  AND actor_can_manage(p.id, sqlc.arg('actor_id')::uuid)
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
WHERE m.user_id = sqlc.arg('person_id')::uuid
  AND actor_can_manage(m.property_id, sqlc.arg('actor_id')::uuid)
ORDER BY m.created_at ASC;

-- name: ListParticipantInvitationsForRemoval :many
SELECT i.id, i.property_id, i.email
FROM property_member_invitations i
WHERE lower(i.email) = lower(sqlc.arg('person_email')::text)
  AND actor_can_manage(i.property_id, sqlc.arg('actor_id')::uuid)
ORDER BY i.created_at ASC;

-- The global «Отозвать доступ всем» (issue #1310) walks the same removal
-- scope without the per-person filter: every membership and every pending
-- invitation on the acting owner's properties, archived included (issue
-- #163). The scope predicate stays the authorization: rows outside it never
-- leave the database.

-- name: ListParticipantMembershipsForRemovalByOwner :many
SELECT m.id, m.property_id, m.user_id, m.status
FROM property_members m
WHERE actor_can_manage(m.property_id, sqlc.arg('actor_id')::uuid)
ORDER BY m.created_at ASC;

-- name: ListParticipantInvitationsForRemovalByOwner :many
SELECT i.id, i.property_id, i.email
FROM property_member_invitations i
WHERE actor_can_manage(i.property_id, sqlc.arg('actor_id')::uuid)
ORDER BY i.created_at ASC;

-- The gate probes of the global revoke (issue #1310): with no target id the
-- endpoint conditions on bare existence. The ownership probe counts any
-- status — archived objects still hold revocable legs; the membership probe
-- splits the non-owner refusal into the participant's loud 403 and the
-- stranger's privacy-404.

-- name: OwnerOwnsAnyProperty :one
SELECT EXISTS (
  SELECT 1 FROM properties WHERE owner_id = sqlc.arg('owner_id')::uuid
) AS owns;

-- name: UserHoldsAnyMembership :one
SELECT EXISTS (
  SELECT 1 FROM property_members WHERE user_id = sqlc.arg('user_id')::uuid
) AS holds;
