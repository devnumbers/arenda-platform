-- The «Участник (владельца)» read model (issue #693): an aggregate over
-- property_members ∪ property_member_invitations with no table of its own
-- (chart decision of map #692). The reading actor's scope is the set of
-- non-archived properties the actor owns or manages as an active full_access
-- member; archived properties stay invisible like everywhere else on the
-- platform (issue #163). The scope doubles as the authorization: rows outside
-- it never leave the database.
--
-- THE MANAGE-SCOPE PREDICATE (the «actor_can_manage» canon): owner OR active
-- full_access is the authorization of the three scope queries in this file
-- (read scope + both removal listings; the two ByProperties listings filter
-- by the already-scoped id list). The predicate lives in one place — the SQL
-- function actor_can_manage
-- (migration 000135, issue #794) — which the queries below call; before #794
-- sqlc could not share the text between query bodies, so the predicate was
-- kept as three byte-identical copies pinned by the matrix test.
-- TestParticipantRepository_ManageScopePredicateMatrix runs the same actor
-- verdict across all three queries and stays the gate: semantic drift of the
-- function fails there instead of opening a silent privacy hole.

-- name: ListParticipantScopeProperties :many
-- The scope rows carry the leg avatar pair (решение #1286, остаток #1244
-- п.3): the property type — the placeholder glyph's key — and the photo's
-- same-origin streaming path (ADR 0065), so the participant read model's
-- legs are self-sufficient and the client needs no /properties join (a cold
-- cache no longer renders BoldHome).
SELECT p.id, p.owner_id, p.name AS title, p.type,
       COALESCE(CASE WHEN p.photo_key IS NOT NULL
                     THEN '/api/v1/properties/' || p.id::text || '/photo'
                     END, '')::text AS photo_path
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
